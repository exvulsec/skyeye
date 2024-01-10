package ethereum

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/ethereum/exporter"
	"go-etl/model"
	"go-etl/utils"
)

type transactionExecutor struct {
	blockExecutor BlockExecutor
	logExecutor   Executor
	blockNumber   int64
	workers       int
	isNastiff     bool
	exporters     []exporter.Exporter
}

func NewTransactionExecutor(blockExecutor BlockExecutor,
	logExecutor Executor,
	chain, openapi string,
	workers int, isNastiff bool) Executor {
	return &transactionExecutor{
		blockExecutor: blockExecutor,
		logExecutor:   logExecutor,
		workers:       workers,
		isNastiff:     isNastiff,
		exporters:     exporter.NewTransactionExporters(chain, openapi, isNastiff, workers),
	}
}

func (te *transactionExecutor) Run() {
	go te.blockExecutor.GetBlocks()
	if te.logExecutor != nil {
		go te.logExecutor.Run()
	}
	exporter.StartExporters(te.exporters)
	for i := 0; i < te.workers; i++ {
		go te.ExtractTransaction()
	}
	select {}
}

func (te *transactionExecutor) GetBlockInfoByBlockNumber(blockNumber uint64) *types.Block {
	for i := 0; i < 6; i++ {
		block, err := te.GetBlockInfoWithTimeOut(blockNumber)
		if err != nil && !utils.IsRetriableError(err) {
			logrus.Errorf("get block %d is err: %v", blockNumber, err)
			break
		}
		if block != nil {
			return block
		}
		time.Sleep(1 * time.Second)
		logrus.Infof("retry %d times to get block %d", i+1, blockNumber)
	}
	logrus.Errorf("get block %d failed, drop it", blockNumber)
	return nil
}

func (te *transactionExecutor) GetBlockInfoWithTimeOut(blockNumber uint64) (*types.Block, error) {
	retryContextTimeout, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer retryCancel()
	block, err := client.EvmClient().BlockByNumber(retryContextTimeout, big.NewInt(int64(blockNumber)))
	return block, err
}

func (te *transactionExecutor) ExtractTransaction() {
	for blockNumber := range te.blockExecutor.blocks {
		block := te.GetBlockInfoByBlockNumber(blockNumber)
		if block != nil {
			logrus.Infof("start to extract transaction infos from the block: %d infos", blockNumber)
			te.blockNumber = block.Number().Int64()
			transactions := te.ExtractByBlock(*block)
			enrichTxs := te.enrichContractCreation(transactions)
			exporter.WriteDataToExporters(te.exporters, enrichTxs)
		}
	}
}

func (te *transactionExecutor) ExtractByBlock(block types.Block) model.Transactions {
	startTimestamp := time.Now()
	txs := model.Transactions{}
	rwMutex := sync.RWMutex{}
	wg := sync.WaitGroup{}

	for index, tx := range block.Transactions() {
		if tx == nil {
			continue
		}
		transaction := tx
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			t := model.Transaction{}
			t.ConvertFromBlock(transaction)
			t.BlockNumber = block.Number().Int64()
			t.BlockTimestamp = int64(block.Time())
			if block.BaseFee() != nil {
				t.GasBase = decimal.NewFromBigInt(block.BaseFee(), 0)
			}
			rwMutex.Lock()
			txs = append(txs, t)
			rwMutex.Unlock()
		}(index)
	}
	wg.Wait()
	logrus.Infof("extract %d txs from block %d cost %.2fs",
		len(block.Transactions()),
		block.Number(),
		time.Since(startTimestamp).Seconds())
	return txs
}

func (te *transactionExecutor) enrichContractCreation(items model.Transactions) model.Transactions {
	txs := model.Transactions{}
	enrichTXs := model.Transactions{}
	for _, item := range items {
		if item.ToAddress == nil {
			enrichTXs = append(enrichTXs, item)
		} else {
			txs = append(txs, item)
		}
	}
	enrichTXs.EnrichReceipts()
	txs = append(txs, enrichTXs...)
	return txs
}

func (te *transactionExecutor) Enrich() {

	if te.logExecutor != nil {
		lge, _ := te.logExecutor.(*logExecutor)
		lge.filterLogsByTopics(te.blockNumber, te.blockNumber)
	}
}

func (te *transactionExecutor) Export() {
	utils.WriteBlockNumberToFile(config.Conf.ETL.PreviousFile, te.blockNumber)
}

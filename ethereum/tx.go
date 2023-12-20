package ethereum

import (
	"context"
	"math/big"
	"strings"
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
	batchSize     int
	items         any
	isNastiff     bool
	exporters     []exporter.Exporter
}

func NewTransactionExecutor(blockExecutor BlockExecutor,
	logExecutor Executor,
	chain, openapi string,
	workers, batchSize int,
	isNastiff bool) Executor {
	return &transactionExecutor{
		blockExecutor: blockExecutor,
		logExecutor:   logExecutor,
		workers:       workers,
		batchSize:     batchSize,
		isNastiff:     isNastiff,
		exporters:     exporter.NewTransactionExporters(chain, openapi, isNastiff),
	}
}

func (te *transactionExecutor) Run() {
	go te.blockExecutor.GetBlocks()
	if te.logExecutor != nil {
		go te.logExecutor.Run()
	}

	for blockNumber := range te.blockExecutor.blocks {
		timeoutContext, _ := context.WithTimeout(context.Background(), 5*time.Second)
		block, err := client.EvmClient().BlockByNumber(timeoutContext, big.NewInt(int64(blockNumber)))
		if err != nil {
			if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "context deadline exceeded") {
				for i := 0; i < 10; i++ {
					time.Sleep(1 * time.Second)
					retryContextTimeout, _ := context.WithTimeout(context.Background(), 5*time.Second)
					logrus.Infof("retry %d to get block: %d info", i+1, blockNumber)
					block, err = client.EvmClient().BlockByNumber(retryContextTimeout, big.NewInt(int64(blockNumber)))
					if err != nil && (!strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "context deadline exceeded")) {
						logrus.Errorf("get block %d info is err: %v, drop it ", blockNumber, err)
						continue
					}
					if block != nil {
						break
					}
				}
			}
		}
		if block != nil {
			logrus.Infof("start to extract transaction infos from the block: %d infos", blockNumber)
			te.blockNumber = block.Number().Int64()
			te.items = te.ExtractByBlock(*block)
			te.Enrich()
			te.Export()
		} else {
			logrus.Errorf("get block %d failed, drop it ", blockNumber)
		}
	}
}

func (te *transactionExecutor) ExtractByBlock(block types.Block) any {
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

func (te *transactionExecutor) Enrich() {
	startTimestamp := time.Now()
	if te.isNastiff {
		te.filterContractCreation()
		txs := te.items.(model.Transactions)
		if len(txs) > 0 {
			txs.EnrichReceipts(te.batchSize)
		}
		logrus.Infof("enrich %d txs from receipt cost: %.2fs", len(txs), time.Since(startTimestamp).Seconds())
	} else {
		if te.logExecutor != nil {
			lge, _ := te.logExecutor.(*logExecutor)
			lge.filterLogsByTopics(te.blockNumber, te.blockNumber)
		}
	}
}

func (te *transactionExecutor) Export() {
	txs := te.items.(model.Transactions)
	if len(txs) > 0 {
		for _, e := range te.exporters {
			e.ExportItems(txs)
		}
	}
	utils.WriteBlockNumberToFile(config.Conf.ETL.PreviousFile, te.blockNumber)
}

func (te *transactionExecutor) filterContractCreation() {
	txs := model.Transactions{}
	for _, item := range te.items.(model.Transactions) {
		if item.ToAddress == nil {
			txs = append(txs, item)
		}
	}
	te.items = txs
}

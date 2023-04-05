package ethereum

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	"go-etl/config"
	"go-etl/ethereum/exporter"
	"go-etl/model"
	"go-etl/utils"
)

type transactionExecutor struct {
	blockExecutor      BlockExecutor
	blockNumber        int64
	workers            int
	batchSize          int
	items              any
	isCreationContract bool
	exporters          []exporter.Exporter
}

func NewTransactionExecutor(blockExecutor BlockExecutor, chain, table string, workers, batchSize, nonce int, isCreationContract, writeToRedis bool) Executor {
	return &transactionExecutor{
		blockExecutor:      blockExecutor,
		workers:            workers,
		batchSize:          batchSize,
		isCreationContract: isCreationContract,
		exporters:          exporter.NewTransactionExporters(chain, table, writeToRedis, nonce),
	}
}

func (te *transactionExecutor) Run() {
	go te.blockExecutor.GetBlocks()
	for block := range te.blockExecutor.blocks {
		te.blockNumber = block.Number().Int64()
		te.items = te.ExtractByBlock(block)
		te.Enrich()
		te.Export()
	}
}

func (te *transactionExecutor) ExtractByBlock(block types.Block) any {
	startTimestamp := time.Now()
	txs := model.Transactions{}
	rwMutex := sync.RWMutex{}
	wg := sync.WaitGroup{}

	for index := range block.Transactions() {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			t := model.Transaction{}
			t.ConvertFromBlock(block.Transactions()[index])
			t.BlockNumber = block.Number().Int64()
			t.BlockTimestamp = int64(block.Time())
			t.GasBase = decimal.NewFromBigInt(block.BaseFee(), 0)
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
	te.filterTransactions()
	txs := te.items.(model.Transactions)
	if len(txs) > 0 {
		txs.EnrichReceipts(te.batchSize, te.workers)
		logrus.Infof("enrich %d txs from receipt cost: %.2fs", len(txs), time.Since(startTimestamp).Seconds())
	}
}

func (te *transactionExecutor) Export() {
	txs := te.items.(model.Transactions)
	if len(txs) > 0 {
		for _, e := range te.exporters {
			e.ExportItems(txs)
		}
	}
	utils.WriteBlockNumberToFile(config.Conf.ETLConfig.PreviousFile, te.blockNumber)
}

func (te *transactionExecutor) filterTransactions() {
	txs := model.Transactions{}
	for _, item := range te.items.(model.Transactions) {
		if te.filterTransaction(item) {
			txs = append(txs, item)
		}
	}
	te.items = txs
}

func (te *transactionExecutor) filterTransaction(tx model.Transaction) bool {
	if !te.isCreationContract {
		return true
	}
	if tx.ToAddress == nil {
		return true
	}
	return false
}

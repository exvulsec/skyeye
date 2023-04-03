package ethereum

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	"go-etl/config"
	"go-etl/database"
	"go-etl/model"
	"go-etl/utils"
)

type transactionExecutor struct {
	blockNumber int64
	blocks      chan types.Block
	workers     int
	batchSize   int
	items       any
	filter      filter
}

type filter struct {
	nonce              int
	isCreationContract bool
}

func NewTransactionExecutor(blocks chan types.Block, workers, batchSize, nonce int, isCreationContract bool) Executor {
	return &transactionExecutor{
		blocks:    blocks,
		workers:   workers,
		batchSize: batchSize,
		filter: filter{
			nonce:              nonce,
			isCreationContract: isCreationContract,
		},
	}
}

func (te *transactionExecutor) Run() {
	for block := range te.blocks {
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
	txs.EnrichReceipts(te.batchSize, te.workers)
	logrus.Infof("enrich %d txs from receipt cost: %.2fs", len(txs), time.Since(startTimestamp).Seconds())
}

func (te *transactionExecutor) Export() {
	startTimestamp := time.Now()
	txs := te.items.(model.Transactions)
	txs.CreateBatchToDB(utils.ComposeTableName(
		config.Conf.ETLConfig.Chain,
		database.TableTransactions),
		config.Conf.Postgresql.MaxOpenConns,
	)
	logrus.Infof("insert %d txs into database cost: %.2f", len(txs), time.Since(startTimestamp).Seconds())
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
	if te.filter.isCreationContract {
		if tx.ToAddress == nil && tx.Nonce < te.filter.nonce {
			return true
		}
		return false
	}
	if tx.Nonce < te.filter.nonce {
		return true
	}
	return false
}

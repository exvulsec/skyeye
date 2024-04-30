package extractor

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/exporter"
	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/task"
	"github.com/exvulsec/skyeye/utils"
)

type transactionExtractor struct {
	blocks        chan uint64
	transactionCh chan transactionChan
	latestBlock   *uint64
	workers       int
	exporters     []exporter.Exporter
	monitorAddrs  model.MonitorAddrs
}

type transactionChan struct {
	transactions model.Transactions
	blocks       uint64
}

func NewTransactionExtractor(workers int) Extractor {
	monitorAddrs := model.MonitorAddrs{}
	if err := monitorAddrs.List(); err != nil {
		logrus.Panicf("list monitor addr is err %v", err)
	}
	return &transactionExtractor{
		blocks:        make(chan uint64, 10),
		transactionCh: make(chan transactionChan, 100),
		workers:       workers,
		monitorAddrs:  monitorAddrs,
	}
}

func (te *transactionExtractor) Run() {
	go te.ProcessTasks()
	te.ExtractLatestBlocks()
}

func (te *transactionExtractor) ProcessTasks() {
	for txsCh := range te.transactionCh {
		te.ExecuteTransaction(txsCh.blocks, txsCh.transactions)
	}
}

func (te *transactionExtractor) ExtractLatestBlocks() {
	for blockNumber := range te.blocks {
		te.transactionCh <- transactionChan{transactions: te.extractTransactionFromBlock(blockNumber), blocks: blockNumber}
	}
}

func (te *transactionExtractor) ExecuteTransaction(blockNumber uint64, txs model.Transactions) {
	tasks := []task.Task{task.NewContractTask(te.monitorAddrs), task.NewAssetSubTask(te.monitorAddrs)}
	for _, t := range tasks {
		t.Do(txs)
	}
	for _, e := range te.exporters {
		e.Export(blockNumber)
	}
}

func (te *transactionExtractor) Extract(data any) {
	block, ok := data.(uint64)
	if ok {
		if te.latestBlock == nil {
			te.latestBlock = &block
			go te.extractPreviousBlocks()
		}
		te.blocks <- block
	}
}

func (te *transactionExtractor) extractTransactionFromBlock(blockNumber uint64) model.Transactions {
	fn := func(element any) (any, error) {
		retryContextTimeout, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer retryCancel()
		blkNumber, ok := element.(uint64)
		if !ok {
			return nil, errors.New("block number's type is not uint64")
		}
		return client.MultiEvmClient()[config.Conf.ETL.Chain].BlockByNumber(retryContextTimeout, big.NewInt(int64(blkNumber)))
	}

	block, ok := utils.Retry(blockNumber, fn).(*types.Block)
	if ok {
		return te.convertTransactionFromBlock(block)
	}
	return model.Transactions{}
}

func (te *transactionExtractor) extractPreviousBlocks() {
	previousBlockNumber := utils.GetBlockNumberFromFile(config.Conf.ETL.PreviousFile)
	latestBlockNumber := *te.latestBlock
	if previousBlockNumber == 0 {
		previousBlockNumber = latestBlockNumber - 1
	}
	te.exporters = append(te.exporters, exporter.NewBlockToFileExporter(previousBlockNumber))
	previousBlockNumber += 1
	for blockNumber := previousBlockNumber; blockNumber < latestBlockNumber; blockNumber++ {
		te.transactionCh <- transactionChan{transactions: te.extractTransactionFromBlock(blockNumber), blocks: blockNumber}
	}
	logrus.Infof("processed the previous blocks")
}

func (te *transactionExtractor) convertTransactionFromBlock(block *types.Block) model.Transactions {
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
			defer func() {
				wg.Done()
			}()
			t := model.Transaction{}
			t.ConvertFromBlock(transaction)
			t.BlockNumber = block.Number().Int64()
			t.BlockTimestamp = int64(block.Time())
			rwMutex.Lock()
			txs = append(txs, t)
			rwMutex.Unlock()
		}(index)
	}
	wg.Wait()
	logrus.Infof("extract %d transactions from block %d cost %.2fs",
		len(block.Transactions()),
		block.Number(),
		time.Since(startTimestamp).Seconds())
	return txs
}

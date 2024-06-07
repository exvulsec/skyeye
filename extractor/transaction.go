package extractor

import (
	"context"
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
	monitorAddrs  *model.MonitorAddrs
}

type transactionChan struct {
	transactions model.Transactions
	block        uint64
}

func NewTransactionExtractor(workers int) Extractor {
	monitorAddrs := model.MonitorAddrs{}
	if err := monitorAddrs.List(); err != nil {
		logrus.Panicf("list monitor addr is err %v", err)
	}
	return &transactionExtractor{
		blocks:        make(chan uint64, 100),
		transactionCh: make(chan transactionChan, 1000),
		workers:       workers,
		monitorAddrs:  &monitorAddrs,
	}
}

func (te *transactionExtractor) Run() {
	go te.ProcessTasks()
	te.ExtractBlocks()
}

func (te *transactionExtractor) ProcessTasks() {
	concurrency := make(chan struct{}, te.workers)
	for txsCh := range te.transactionCh {
		concurrency <- struct{}{}
		go func() {
			defer func() { <-concurrency }()
			te.ExecuteTask(txsCh)
		}()

	}
}

func (te *transactionExtractor) ExtractBlocks() {
	concurrency := make(chan struct{}, te.workers)
	for blockNumber := range te.blocks {
		concurrency <- struct{}{}
		go func() {
			defer func() { <-concurrency }()
			te.extractTransactionFromBlock(blockNumber)
		}()
	}
}

func (te *transactionExtractor) ExecuteTask(txCh transactionChan) {
	tasks := []task.Task{
		task.NewContractTask(te.monitorAddrs),
		task.NewAssetTask(te.monitorAddrs),
	}
	var data any = txCh.transactions
	for _, t := range tasks {
		data = t.Run(data)
	}
	for _, e := range te.exporters {
		e.Export(txCh.block)
	}
}

func (te *transactionExtractor) Extract(data any) {
	header, ok := data.(*types.Header)
	if ok {
		block := header.Number.Uint64()
		if te.latestBlock == nil {
			te.latestBlock = &block
			go te.extractPreviousBlocks()
		}
		te.blocks <- block
	}
}

func (te *transactionExtractor) extractTransactionFromBlock(blockNumber uint64) {
	startTime := time.Now()

	block, ok := utils.Retry(func() (any, error) {
		retryContextTimeout, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer retryCancel()
		return client.MultiEvmClient()[config.Conf.ETL.Chain].BlockByNumber(retryContextTimeout, big.NewInt(int64(blockNumber)))
	}).(*types.Block)

	if ok {
		te.transactionCh <- transactionChan{transactions: te.convertTransactionFromBlock(block), block: blockNumber}

		logrus.Infof("block: %d, extract transactions: %d, elapsed: %s",
			block.Number(),
			len(block.Transactions()),
			utils.ElapsedTime(startTime))
	}
}

func (te *transactionExtractor) extractPreviousBlocks() {
	startBlock := utils.GetBlockNumberFromFile(config.Conf.ETL.PreviousFile)
	endBlock := *te.latestBlock
	if startBlock == 0 {
		startBlock = endBlock - 1
	} else if endBlock-config.Conf.ETL.PreviousBlockThreshold-1 > startBlock {
		startBlock = endBlock - config.Conf.ETL.PreviousBlockThreshold - 1
	}
	te.exporters = append(te.exporters, exporter.NewBlockToFileExporter(startBlock))
	startBlock += 1

	logrus.Infof("process the previous blocks from %d to %d", startBlock, endBlock)
	for blockNumber := startBlock; blockNumber < endBlock; blockNumber++ {
		te.blocks <- blockNumber
	}
	logrus.Infof("process the previous blocks is finished.")
}

func (te *transactionExtractor) convertTransactionFromBlock(block *types.Block) model.Transactions {
	txs := model.Transactions{}
	rwMutex := sync.RWMutex{}
	wg := sync.WaitGroup{}
	concurrency := make(chan struct{}, te.workers)
	for _, tx := range block.Transactions() {
		if tx == nil {
			continue
		}
		transaction := tx
		wg.Add(1)
		concurrency <- struct{}{}
		go func() {
			defer func() {
				wg.Done()
				<-concurrency
			}()
			t := model.Transaction{}
			t.ConvertFromBlock(transaction)
			t.BlockNumber = block.Number().Int64()
			t.BlockTimestamp = int64(block.Time())
			rwMutex.Lock()
			txs = append(txs, t)
			rwMutex.Unlock()
		}()
	}
	wg.Wait()
	return txs
}

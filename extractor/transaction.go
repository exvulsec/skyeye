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
	concurrency := make(chan struct{}, te.workers)
	for txsCh := range te.transactionCh {
		concurrency <- struct{}{}
		go func() {
			defer func() { <-concurrency }()
			te.ExecuteTransaction(txsCh.blocks, txsCh.transactions)
		}()

	}
}

func (te *transactionExtractor) ExtractLatestBlocks() {
	concurrency := make(chan struct{}, te.workers)
	for blockNumber := range te.blocks {
		concurrency <- struct{}{}
		go func() {
			defer func() { <-concurrency }()
			te.transactionCh <- transactionChan{transactions: te.extractTransactionFromBlock(blockNumber), blocks: blockNumber}
		}()
	}
}

func (te *transactionExtractor) ExecuteTransaction(blockNumber uint64, txs model.Transactions) {
	tasks := []task.Task{task.NewContractTask(te.monitorAddrs), task.NewAssetSubTask(te.monitorAddrs)}
	var data any = txs
	for _, t := range tasks {
		data = t.Do(data)
	}
	for _, e := range te.exporters {
		e.Export(blockNumber)
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
	startTime := time.Now()
	block, ok := utils.Retry(blockNumber, fn).(*types.Block)
	if ok {
		txs := te.convertTransactionFromBlock(block)
		logrus.Infof("block: %d, extract transactions: %d, elapsed: %s",
			block.Number(),
			len(block.Transactions()),
			utils.ElapsedTime(startTime))
		return txs
	}

	return model.Transactions{}
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

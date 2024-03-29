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
	"github.com/exvulsec/skyeye/executor"
	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type TransactionExtractor struct {
	workers             int
	previousBlockNumber chan uint64
	previousDone        chan bool
	latestBlockNumbers  chan uint64
	txsCh               model.Transactions
	executors           []executor.Executor
}

func NewTransactionExtractor(workers int) *TransactionExtractor {
	return &TransactionExtractor{
		workers:             workers,
		previousBlockNumber: make(chan uint64, 1),
		previousDone:        make(chan bool, 1),
		latestBlockNumbers:  make(chan uint64, 100),
		executors:           []executor.Executor{executor.NewTransactionExecutor()},
	}
}

func (te *TransactionExtractor) extractTransactions() {
	go te.extractPreviousBlocks()
	te.subscribeLatestBlocks()
}

func (te *TransactionExtractor) extractPreviousBlocks() {
	previousBlockNumber := utils.GetBlockNumberFromFile(config.Conf.ETL.PreviousFile)
	latestBlockNumber := <-te.previousBlockNumber
	if previousBlockNumber == 0 {
		previousBlockNumber = latestBlockNumber - 1
	}
	previousBlockNumber += 1
	for previousBlockNumber < latestBlockNumber {
		logrus.Infof("extract transaction from previous block number %d", previousBlockNumber)
		te.sendItemsToExporters(te.extractTransactionFromBlock(previousBlockNumber))
		previousBlockNumber++
	}
	te.previousDone <- true
}

func (te *TransactionExtractor) extractLatestBlocks() {
	for blockNumber := range te.latestBlockNumbers {
		te.sendItemsToExporters(te.extractTransactionFromBlock(blockNumber))
	}
}

func (te *TransactionExtractor) subscribeLatestBlocks() {
	headers := make(chan *types.Header)

	sub, err := client.EvmClient().SubscribeNewHead(context.Background(), headers)
	if err != nil {
		panic(err)
	}
	perviousOnce := sync.Once{}
	logrus.Info("subscribe to the latest blocks...")
	for {
		select {
		case err = <-sub.Err():
			sub.Unsubscribe()
			logrus.Fatalf("subscription block is error: %v", err)
			close(te.previousDone)
		case header := <-headers:
			logrus.Infof("received a new header: %d", header.Number.Uint64())
			perviousOnce.Do(func() {
				te.previousBlockNumber <- header.Number.Uint64()
				close(te.previousBlockNumber)
			})
			te.latestBlockNumbers <- header.Number.Uint64()
		case <-te.previousDone:
			go te.extractLatestBlocks()
		}
	}
}

func (te *TransactionExtractor) extractTransactionFromBlock(blockNumber uint64) model.Transactions {
	fn := func(element any) (any, error) {
		retryContextTimeout, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer retryCancel()
		blkNumber, ok := element.(uint64)
		if !ok {
			return nil, errors.New("block number's type is not uint64")
		}
		return client.EvmClient().BlockByNumber(retryContextTimeout, big.NewInt(int64(blkNumber)))
	}

	block := utils.Retry(10, blockNumber, fn).(*types.Block)
	if block == nil {
		return nil
	}
	logrus.Infof("start to extract %d transactions from block %d", block.Transactions().Len(), blockNumber)
	return te.convertTransactionFromBlock(block)
}

func (te *TransactionExtractor) convertTransactionFromBlock(block *types.Block) model.Transactions {
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

func (te *TransactionExtractor) Run() {
	for _, exec := range te.executors {
		for index := range te.workers {
			logrus.Infof("thread %d: start %s executor", index+1, exec.Name())
			go exec.Execute()
		}
	}
	te.extractTransactions()
}

func (te *TransactionExtractor) sendItemsToExporters(txs model.Transactions) {
	for _, exec := range te.executors {
		exec.GetItemsCh() <- txs
	}
}

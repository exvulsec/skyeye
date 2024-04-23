package executor

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type transactionExecutor struct {
	items     chan any
	workers   int
	executors []Executor
}

func NewTransactionExtractor(workers int, latestBlockNumberCh chan int64) Executor {
	return &transactionExecutor{
		items:     make(chan any),
		workers:   workers,
		executors: []Executor{NewContractExecutor(workers, latestBlockNumberCh)},
	}
}

func (te *transactionExecutor) Name() string {
	return "Transaction"
}

func (te *transactionExecutor) GetItemsCh() chan any {
	return te.items
}

func (te *transactionExecutor) Execute() {
	for _, exec := range te.executors {
		go exec.Execute()
	}
	for range te.workers {
		go func() {
			for item := range te.items {
				blockNumber, ok := item.(uint64)
				if ok {
					txs := te.extractTransactionFromBlock(blockNumber)
					for _, executor := range te.executors {
						executor.GetItemsCh() <- txs
					}
				}
			}
		}()
	}
}

func (te *transactionExecutor) extractTransactionFromBlock(blockNumber uint64) model.Transactions {
	fn := func(element any) (any, error) {
		retryContextTimeout, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer retryCancel()
		blkNumber, ok := element.(uint64)
		if !ok {
			return nil, errors.New("block number's type is not uint64")
		}
		return client.EvmClient().BlockByNumber(retryContextTimeout, big.NewInt(int64(blkNumber)))
	}

	block, ok := utils.Retry(blockNumber, fn).(*types.Block)
	if ok {
		logrus.Infof("start to extract %d transactions from block %d", block.Transactions().Len(), blockNumber)
		return te.convertTransactionFromBlock(block)
	}
	return model.Transactions{}
}

func (te *transactionExecutor) convertTransactionFromBlock(block *types.Block) model.Transactions {
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

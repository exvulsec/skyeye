package executor

import (
	"context"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/utils"
)

type blockExecutor struct {
	items              chan any
	latestBlock        *types.Header
	previousDone       chan bool
	latestBlockNumbers chan uint64
	executors          []Executor
}

func NewBlockExecutor() Executor {
	return &blockExecutor{
		latestBlock:        &types.Header{},
		previousDone:       make(chan bool, 1),
		latestBlockNumbers: make(chan uint64, 100),
		executors:          []Executor{NewTransactionExtractor()},
	}
}

func (be *blockExecutor) Name() string {
	return "Block"
}

func (be *blockExecutor) GetItemsCh() chan any {
	return be.items
}

func (be *blockExecutor) Execute() {
	startPrevious := make(chan bool, 1)
	go be.extractPreviousBlocks(startPrevious)
	be.subscribeLatestBlocks(startPrevious)
}

func (be *blockExecutor) extractPreviousBlocks(startPrevious chan bool) {
	<-startPrevious
	previousBlockNumber := utils.GetBlockNumberFromFile(config.Conf.ETL.PreviousFile)
	latestBlockNumber := be.latestBlock.Number.Uint64()
	if previousBlockNumber == 0 {
		previousBlockNumber = latestBlockNumber - 1
	}
	previousBlockNumber += 1
	for blockNumber := previousBlockNumber; blockNumber < latestBlockNumber; blockNumber++ {
		logrus.Infof("extract transaction from block number %d", blockNumber)
		be.sendItemsToExecutors(blockNumber)
	}
	be.previousDone <- true
}

func (be *blockExecutor) extractLatestBlocks() {
	for blockNumber := range be.latestBlockNumbers {
		logrus.Infof("received a new block from header: %d", blockNumber)
		be.sendItemsToExecutors(blockNumber)
	}
}

func (be *blockExecutor) subscribeLatestBlocks(startPrevious chan bool) {
	headers := make(chan *types.Header)

	sub, err := client.EvmClient().SubscribeNewHead(context.Background(), headers)
	if err != nil {
		panic(err)
	}
	logrus.Info("subscribe to the latest blocks...")
	for {
		select {
		case err = <-sub.Err():
			sub.Unsubscribe()
			logrus.Fatalf("subscription block is error: %v", err)
			close(be.previousDone)
		case header := <-headers:
			if be.latestBlock == nil {
				startPrevious <- true
				be.latestBlock = header
			}
			be.latestBlockNumbers <- header.Number.Uint64()
		case <-be.previousDone:
			go be.extractLatestBlocks()
		}
	}
}

func (be *blockExecutor) sendItemsToExecutors(items any) {
	for _, executor := range be.executors {
		executor.GetItemsCh() <- items
	}
}

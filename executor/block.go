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
	items                     chan any
	latestBlock               *uint64
	previousDone              chan bool
	latestBlockNumbers        chan uint64
	fileLatestBlockNumberChan chan int64
	executors                 []Executor
	fileExecutor              Executor
}

func NewBlockExecutor(workers int) Executor {
	fileLatestChan := make(chan int64, 1)
	return &blockExecutor{
		previousDone:              make(chan bool, 1),
		latestBlockNumbers:        make(chan uint64, 100),
		fileLatestBlockNumberChan: fileLatestChan,
		executors:                 []Executor{NewTransactionExtractor(workers, fileLatestChan)},
	}
}

func (be *blockExecutor) Name() string {
	return "Block"
}

func (be *blockExecutor) GetItemsCh() chan any {
	return be.items
}

func (be *blockExecutor) Execute() {
	for _, exec := range be.executors {
		go exec.Execute()
	}
	startPrevious := make(chan bool, 1)
	go be.extractPreviousBlocks(startPrevious)
	be.subscribeLatestBlocks(startPrevious)
}

func (be *blockExecutor) extractPreviousBlocks(startPrevious chan bool) {
	select {
	case <-startPrevious:
		previousBlockNumber := utils.GetBlockNumberFromFile(config.Conf.ETL.PreviousFile)
		latestBlockNumber := *be.latestBlock
		if previousBlockNumber == 0 {
			previousBlockNumber = latestBlockNumber - 1
		}
		be.fileLatestBlockNumberChan <- int64(previousBlockNumber)
		previousBlockNumber += 1
		for blockNumber := previousBlockNumber; blockNumber < latestBlockNumber; blockNumber++ {
			logrus.Infof("extract transaction from block number %d", blockNumber)
			be.sendItemsToExecutors(blockNumber)
		}
		be.previousDone <- true
	}
}

func (be *blockExecutor) extractLatestBlocks() {
	for blockNumber := range be.latestBlockNumbers {
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
			blockNumber := header.Number.Uint64()
			logrus.Infof("received a new block from header: %d", blockNumber)
			if be.latestBlock == nil {
				startPrevious <- true
				be.latestBlock = &blockNumber
			}
			be.latestBlockNumbers <- blockNumber
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

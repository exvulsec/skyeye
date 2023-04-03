package ethereum

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/event"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/utils"
)

func GetLatestBlocks() chan types.Block {
	blocks := make(chan types.Block, 100)
	blockNumbers := make(chan int64, 100)
	go getLatestBlocks(blockNumbers)
	go getBlockInfo(blocks, blockNumbers)
	return blocks
}

var latestBlockNumber uint64

func getBlockInfo(blocks chan types.Block, blockNumbers chan int64) {
	for blockNumber := range blockNumbers {
		block, err := client.EvmClient().BlockByNumber(context.Background(), big.NewInt(blockNumber))
		if err != nil {
			logrus.Errorf("get the block by number %d is err %v", blockNumber, err)
			break
		}
		blocks <- *block
	}
	close(blocks)
}

func getPreviousBlocks(blockNumbers chan int64) {
	var err error
	previousBlockNumber := utils.ReadBlockNumberFromFile(config.Conf.ETLConfig.PreviousFile)
	latestBlockNumber, err = client.EvmClient().BlockNumber(context.Background())
	if err != nil {
		logrus.Infof("failed to get to new head block: %v, retry after 5s", err)
		return
	}
	logrus.Infof("latest block number is %d", latestBlockNumber)
	var currentBlockNumber = previousBlockNumber + 1
	for currentBlockNumber <= int64(latestBlockNumber) {
		blockNumbers <- currentBlockNumber
		currentBlockNumber = currentBlockNumber + 1
	}
}

func getLatestBlocks(blockNumbers chan int64) {
	wsClient := client.NewWebSocketClient()
	headers := make(chan *types.Header, 10)
	sub := event.Resubscribe(2*time.Second, func(ctx context.Context) (event.Subscription, error) {
		go getPreviousBlocks(blockNumbers)
		return wsClient.SubscribeNewHead(context.Background(), headers)
	})
	logrus.Infof("subscribed to new head")
	for {
		select {
		case err := <-sub.Err():
			logrus.Errorf("subscription error: %v\n", err)
			sub.Unsubscribe()
			break
		case header := <-headers:
			if header.Number.Uint64() <= latestBlockNumber {
				continue
			}
			logrus.Infof("get new block number %d from subscribed", header.Number.Int64())
			blockNumbers <- header.Number.Int64()
		}
	}
}

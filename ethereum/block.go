package ethereum

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/utils"
)

type BlockExecutor struct {
	chain               string
	processPreviousDone bool
	latestBlocks        chan uint64
	blocks              chan uint64
	latestBlockNumber   uint64
}

func NewBlockExecutor(chain string) BlockExecutor {
	return BlockExecutor{
		chain:               chain,
		processPreviousDone: false,
		latestBlocks:        make(chan uint64, 10000),
		blocks:              make(chan uint64, 10000),
		latestBlockNumber:   0,
	}
}

func (be *BlockExecutor) GetBlocks() {
	be.subNewHeader()
}

func (be *BlockExecutor) setBlocksToChannel(start, end uint64) {
	for currentBlockNumber := start; currentBlockNumber <= end; currentBlockNumber++ {
		logrus.Infof("send updated block %d to channel", currentBlockNumber)
		be.blocks <- currentBlockNumber
	}
}

func (be *BlockExecutor) getPreviousBlocks() {
	var err error

	previousBlockNumber := utils.GetBlockNumberFromDB(be.chain)

	logrus.Infof("previous block number is %d", previousBlockNumber)

	be.latestBlockNumber, err = client.EvmClient().BlockNumber(context.Background())
	if err != nil {
		logrus.Fatalf("failed to get the latest Block number %v", err)
		return
	}

	if previousBlockNumber == 0 {
		previousBlockNumber = be.latestBlockNumber - 1
	}

	logrus.Infof("latest block number is %d", be.latestBlockNumber)
	if previousBlockNumber < be.latestBlockNumber {
		logrus.Infof("start to sync from block %d to block %d", previousBlockNumber+1, be.latestBlockNumber)
		be.setBlocksToChannel(previousBlockNumber+1, be.latestBlockNumber)
	}
	be.processPreviousDone = true
	close(be.latestBlocks)
}

func (be *BlockExecutor) processHeader(header *types.Header) {
	if header.Number.Uint64() <= be.latestBlockNumber {
		return
	}
	newBlockNumber := header.Number.Uint64()
	logrus.Infof("receive a new block %d", newBlockNumber)
	if be.processPreviousDone {
		for blockNumber := range be.latestBlocks {
			logrus.Infof("send receive block %d to channel", blockNumber)
			be.blocks <- blockNumber
		}
		be.blocks <- newBlockNumber
	} else {
		be.latestBlocks <- newBlockNumber
	}
}

func (be *BlockExecutor) subNewHeader() {
	headers := make(chan *types.Header)
	sub := event.Resubscribe(10*time.Second, func(ctx context.Context) (event.Subscription, error) {
		be.latestBlocks = make(chan uint64, 10000)
		be.processPreviousDone = false
		go be.getPreviousBlocks()
		return client.EvmClient().SubscribeNewHead(context.Background(), headers)
	})
	logrus.Info("listing the new block...")
	for {
		select {
		case err := <-sub.Err():
			sub.Unsubscribe()
			close(headers)
			close(be.blocks)
			logrus.Fatalf("subscription block is error: %v", err)
		case header := <-headers:
			be.processHeader(header)
		}
	}
}

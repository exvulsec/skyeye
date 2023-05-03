package ethereum

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/utils"
)

type BlockExecutor struct {
	chain        string
	batchSize    int
	workerSize   int
	latestBlocks chan uint64
	blocks       chan uint64
	running      bool
}

func NewBlockExecutor(chain string, batchSize, workerSize int) BlockExecutor {
	return BlockExecutor{
		chain:      chain,
		batchSize:  batchSize,
		workerSize: workerSize,
		blocks:     make(chan uint64, 10000),
		running:    false,
	}
}

func (be *BlockExecutor) GetBlocks() {
	be.subNewHeader()
}

var latestBlockNumber uint64

func (be *BlockExecutor) getPreviousBlocks() {
	var err error
	previousBlockNumber := uint64(utils.ReadBlockNumberFromFile(config.Conf.ETL.PreviousFile))
	logrus.Infof("previous block number is %d", previousBlockNumber)
	latestBlockNumber, err = client.EvmClient().BlockNumber(context.Background())
	if err != nil {
		logrus.Fatal("failed to get to the latest Block number", err)
		return
	}
	logrus.Infof("latest block number is %d", latestBlockNumber)
	if previousBlockNumber < latestBlockNumber {
		currentBlockNumber := previousBlockNumber + 1
		logrus.Infof("start to sync from block %d to block %d", currentBlockNumber, latestBlockNumber)
		for currentBlockNumber <= latestBlockNumber {
			logrus.Infof("set previous %d blocks to channel", currentBlockNumber)
			be.blocks <- currentBlockNumber
			currentBlockNumber += 1
		}
	}

	be.running = true
	close(be.latestBlocks)

}
func (be *BlockExecutor) subNewHeader() {
	headers := make(chan *types.Header, 10)
	sub := event.Resubscribe(2*time.Second, func(ctx context.Context) (event.Subscription, error) {
		be.running = false
		be.latestBlocks = make(chan uint64, 10000)
		go be.getPreviousBlocks()
		return client.EvmClient().SubscribeNewHead(context.Background(), headers)
	})

	for {
		select {
		case err := <-sub.Err():
			sub.Unsubscribe()
			close(headers)
			close(be.latestBlocks)
			close(be.blocks)
			logrus.Fatalf("subscription block is error: %v", err)
		case header := <-headers:
			if header.Number.Uint64() <= latestBlockNumber {
				continue
			}
			logrus.Infof("receive new block %d", header.Number.Uint64())
			if be.running {
				for blockNumber := range be.latestBlocks {
					logrus.Infof("set %d blocks to channel", blockNumber)
					be.blocks <- blockNumber
				}
				be.blocks <- header.Number.Uint64()
			} else {
				be.latestBlocks <- header.Number.Uint64()
			}
		}
	}
}

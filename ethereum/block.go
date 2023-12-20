package ethereum

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/sirupsen/logrus"

	"go-etl/client"
)

type BlockExecutor struct {
	chain      string
	batchSize  int
	workerSize int
	blocks     chan uint64
	running    bool
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

func (be *BlockExecutor) subNewHeader() {
	headers := make(chan *types.Header, 10)
	sub := event.Resubscribe(2*time.Second, func(ctx context.Context) (event.Subscription, error) {
		be.running = false
		return client.EvmClient().SubscribeNewHead(context.Background(), headers)
	})

	for {
		select {
		case err := <-sub.Err():
			sub.Unsubscribe()
			close(headers)
			close(be.blocks)
			logrus.Fatalf("subscription block is error: %v", err)
		case header := <-headers:
			logrus.Infof("receive new block %d", header.Number.Uint64())
			be.blocks <- header.Number.Uint64()
		}
	}
}

package ethereum

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/utils"
)

type BlockExecutor struct {
	chain        string
	batchSize    int
	workerSize   int
	latestBlocks chan *json.RawMessage
	blocks       chan *json.RawMessage
	running      bool
}

func NewBlockExecutor(chain string, batchSize, workerSize int) BlockExecutor {
	return BlockExecutor{
		chain:      chain,
		batchSize:  batchSize,
		workerSize: workerSize,
		blocks:     make(chan *json.RawMessage, 10000),
		running:    false,
	}
}

func (be *BlockExecutor) GetBlocks() {
	be.subNewHeader()
}

var latestBlockNumber uint64

func (be *BlockExecutor) getPreviousBlocks() {
	var err error
	previousBlockNumber := uint64(utils.ReadBlockNumberFromFile(config.Conf.ETLConfig.PreviousFile))
	logrus.Infof("previous block number is %d", previousBlockNumber)
	latestBlockNumber, err = ethclient.NewClient(client.RPCClient()).BlockNumber(context.Background())
	if err != nil {
		logrus.Fatal("failed to get to the latest Block number", err)
		return
	}
	logrus.Infof("latest block number is %d", latestBlockNumber)
	if previousBlockNumber < latestBlockNumber {
		currentBlockNumber := previousBlockNumber + 1
		logrus.Infof("start to sync from block %d to block %d", currentBlockNumber, latestBlockNumber)
		for currentBlockNumber <= latestBlockNumber {
			logrus.Infof("get %d blocks from batch call", currentBlockNumber)
			client.MultiCall([]rpc.BatchElem{{
				Method: utils.RPCNameEthGetBlockByNumber,
				Args:   []any{hexutil.EncodeUint64(currentBlockNumber), true},
				Result: &json.RawMessage{},
			}}, be.batchSize, be.workerSize, be.blocks)
			currentBlockNumber += 1
		}
	}

	be.running = true
	close(be.latestBlocks)

}
func (be *BlockExecutor) subNewHeader() {
	headers := make(chan *types.Header, 2)

	sub := event.Resubscribe(2*time.Second, func(ctx context.Context) (event.Subscription, error) {
		be.running = false
		be.latestBlocks = make(chan *json.RawMessage, 10000)
		go be.getPreviousBlocks()
		return client.EvmClient().SubscribeNewHead(context.Background(), headers)
	})

	for {
		select {
		case err := <-sub.Err():
			close(headers)
			close(be.latestBlocks)
			close(be.blocks)
			sub.Unsubscribe()
			logrus.Fatalf("subscription block is error: %v", err)
			break
		case header := <-headers:
			if header.Number.Uint64() <= latestBlockNumber {
				continue
			}
			logrus.Infof("receive new block %d", header.Number.Uint64())
			calls := []rpc.BatchElem{{
				Method: utils.RPCNameEthGetBlockByNumber,
				Args:   []any{hexutil.EncodeBig(header.Number), true},
				Result: &json.RawMessage{},
			}}

			if be.running {
				for latestBlock := range be.latestBlocks {
					be.blocks <- latestBlock
				}
				client.MultiCall(calls, be.batchSize, be.workerSize, be.blocks)
			} else {
				client.MultiCall(calls, be.batchSize, be.workerSize, be.latestBlocks)
			}
		}
	}
}

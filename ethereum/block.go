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
	etlRPC "go-etl/rpc"
	"go-etl/utils"
)

type BlockExecutor struct {
	batchSize    int
	workerSize   int
	latestBlocks chan types.Block
	blocks       chan types.Block
	running      bool
}

func NewBlockExecutor(batchSize, workerSize int) BlockExecutor {
	return BlockExecutor{
		batchSize:  batchSize,
		workerSize: workerSize,
		blocks:     make(chan types.Block, 10000),
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
		calls := []rpc.BatchElem{}
		for currentBlockNumber <= latestBlockNumber {
			calls = append(calls, rpc.BatchElem{
				Method: utils.RPCNameEthGetBlockByNumber,
				Args:   []any{hexutil.EncodeUint64(currentBlockNumber), true},
				Result: &json.RawMessage{},
			})
			currentBlockNumber += 1
		}
		if len(calls) > 0 {
			logrus.Infof("get %d blocks from  batch call", len(calls))
			client.MultiCall(calls, be.batchSize, be.workerSize)
			for _, call := range calls {
				if call.Error == nil {
					result, _ := call.Result.(*json.RawMessage)
					block, err := etlRPC.GetBlock(*result)
					if err != nil {
						logrus.Fatalf("get previous block from raw json message is err: %v", err)
					}
					be.blocks <- *block
				}
			}
		}
	}

	be.running = true
	close(be.latestBlocks)

}
func (be *BlockExecutor) subNewHeader() {
	headers := make(chan *types.Header, 2)
	sub := event.Resubscribe(2*time.Second, func(ctx context.Context) (event.Subscription, error) {
		be.running = false
		be.latestBlocks = make(chan types.Block, 10000)
		go be.getPreviousBlocks()
		return client.EvmClient().SubscribeNewHead(context.Background(), headers)
	})
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
			logrus.Infof("receive new block %d", header.Number.Uint64())
			block, err := client.EvmClient().BlockByNumber(context.Background(), header.Number)
			if err != nil {
				logrus.Fatalf("get block is err: %v", err)
			}
			if be.running {
				for latestBlock := range be.latestBlocks {
					be.blocks <- latestBlock
				}
				be.blocks <- *block
			} else {
				be.latestBlocks <- *block
			}
		}
	}
}

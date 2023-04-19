package ethereum

import (
	"context"
	"encoding/json"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
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
			logrus.Infof("get %d blocks from batch call", len(calls))
			client.MultiCall(calls, be.batchSize, be.workerSize, be.blocks)
		}
	}

	be.running = true
	close(be.latestBlocks)

}
func (be *BlockExecutor) subNewHeader() {
	headers := make(chan *types.Header, 2)
	be.running = false
	be.latestBlocks = make(chan *json.RawMessage, 10000)

	go be.getPreviousBlocks()
	sub, err := client.EvmClient().SubscribeNewHead(context.Background(), headers)
	if err != nil {
		logrus.Fatalf("subscribe new header is err: %v", err)
	}

	defer close(be.latestBlocks)
	defer close(be.blocks)

	for {
		select {
		case err = <-sub.Err():
			logrus.Errorf("subscription block is error: %v\n", err)
			sub.Unsubscribe()
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

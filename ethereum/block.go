package ethereum

import (
	"context"
	"encoding/json"
	"log"
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

func GetLatestBlocks(batchSize, workers int) chan types.Block {
	blocks := make(chan types.Block, 100)
	go getLatestBlocks(blocks, batchSize, workers)
	return blocks
}

var latestBlockNumber uint64

func getPreviousBlocks(blockCh chan types.Block, batchSize int, workers int) {
	var err error
	previousBlockNumber := uint64(utils.ReadBlockNumberFromFile(config.Conf.ETLConfig.PreviousFile) + 1)
	latestBlockNumber, err = ethclient.NewClient(client.RPCClient()).BlockNumber(context.Background())
	if err != nil {
		logrus.Fatal("failed to get to the latest Block number", err)
		return
	}
	logrus.Infof("latest block number is %d", latestBlockNumber)
	if err != nil {
		log.Fatal(err)
	}
	calls := []rpc.BatchElem{}
	for previousBlockNumber < latestBlockNumber {
		calls = append(calls, rpc.BatchElem{
			Method: utils.RPCNameEthGetBlockByNumber,
			Args:   []any{hexutil.EncodeUint64(previousBlockNumber), true},
			Result: &json.RawMessage{},
		})
		previousBlockNumber += 1
	}
	client.MultiCall(calls, batchSize, workers)
	for _, call := range calls {
		if call.Error == nil {
			result, _ := call.Result.(*json.RawMessage)
			block, err := etlRPC.GetBlock(*result)
			if err != nil {
				logrus.Fatalf("get block from raw json message is err: %v", err)
			}
			blockCh <- *block
		}
	}
}

func getLatestBlocks(blockCh chan types.Block, batchSize, workers int) {
	wsClient := client.NewWebSocketClient()
	headers := make(chan *types.Header, 10)
	sub := event.Resubscribe(2*time.Second, func(ctx context.Context) (event.Subscription, error) {
		go getPreviousBlocks(blockCh, batchSize, workers)
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
			calls := []rpc.BatchElem{}
			calls = append(calls, rpc.BatchElem{
				Method: utils.RPCNameEthGetBlockByNumber,
				Args:   []any{hexutil.EncodeUint64(header.Number.Uint64()), true},
				Result: &json.RawMessage{},
			})
			client.MultiCall(calls, batchSize, workers)
			for _, call := range calls {
				if call.Error == nil {
					result, _ := call.Result.(*json.RawMessage)
					block, err := etlRPC.GetBlock(*result)
					if err != nil {
						logrus.Fatalf("get block from raw json message is err: %v", err)
					}
					blockCh <- *block
				}
			}

		}
	}
}

package client

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sirupsen/logrus"

	"go-etl/config"
)

var rpcClient *RPCInstance

type RPCInstance struct {
	initializer func() any
	instance    any
	once        sync.Once
}

func (ri *RPCInstance) Instance() any {
	ri.once.Do(func() {
		ri.instance = ri.initializer()
	})
	return ri.instance
}

func initRPCClient() any {
	client, err := rpc.Dial(config.Conf.ETLConfig.ProviderURL)
	if err != nil {
		logrus.Fatalf("failed to connect provider url %s with rpcClient, err is %v", config.Conf.ETLConfig.ProviderURL, err)
	}
	logrus.Infof("connect to provider with rpcClient is successfully")
	return client
}

func RPCClient() *rpc.Client {
	return rpcClient.Instance().(*rpc.Client)
}

func init() {
	rpcClient = &RPCInstance{initializer: initRPCClient}
}

func MultiCall(calls []rpc.BatchElem, batchSize, workerCount int, channel chan *json.RawMessage) {
	wg := sync.WaitGroup{}
	worker := make(chan int, workerCount)
	count := len(calls) / batchSize
	if len(calls)%batchSize != 0 {
		count += 1
	}
	for i := 0; i < count; i++ {
		worker <- 1
		wg.Add(1)
		go func(index int) {
			startTimestamp := time.Now()
			defer func() {
				wg.Done()
				<-worker
			}()
			startIndex := index * batchSize
			endIndex := (index + 1) * batchSize
			if endIndex > len(calls) {
				endIndex = len(calls)
			}
			if err := RPCClient().BatchCall(calls[startIndex:endIndex]); err != nil {
				logrus.Fatalf("batch call is err %v", err)
				return
			}
			for _, call := range calls[startIndex:endIndex] {
				if call.Error != nil {
					logrus.Fatalf("get block number %v from rpc node is err: %v", call.Args, call.Error)
				}
				if channel != nil {

					channel <- call.Result.(*json.RawMessage)
				}
			}
			logrus.Infof("handle %d calls and cost %.2fs", endIndex-startIndex, time.Since(startTimestamp).Seconds())
		}(i)
	}
	wg.Wait()
}

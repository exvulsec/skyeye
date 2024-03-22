package client

import (
	"sync"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
)

type RPCBatchClient struct {
	Workers chan int
	Client  *rpc.Client
}

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
	client, err := rpc.Dial(config.Conf.ETL.ProviderURL)
	if err != nil {
		logrus.Fatalf("failed to connect provider url %s with rpcClient, err is %v", config.Conf.ETL.ProviderURL, err)
	}
	rpcBatchClient := &RPCBatchClient{
		Workers: make(chan int, config.Conf.ETL.Worker),
		Client:  client,
	}
	logrus.Infof("connect to provider with rpcClient is successfully")
	return rpcBatchClient
}

func RPCClient() *RPCBatchClient {
	return rpcClient.Instance().(*RPCBatchClient)
}

func init() {
	rpcClient = &RPCInstance{initializer: initRPCClient}
}

func (rb *RPCBatchClient) MultiCall(calls []rpc.BatchElem, batchSize int) {
	wg := sync.WaitGroup{}
	count := len(calls) / batchSize
	if len(calls)%batchSize != 0 {
		count += 1
	}
	for i := 0; i < count; i++ {
		rb.Workers <- 1
		wg.Add(1)
		go func(index int) {
			defer func() {
				wg.Done()
				<-rb.Workers
			}()
			startIndex := index * batchSize
			endIndex := (index + 1) * batchSize
			if endIndex > len(calls) {
				endIndex = len(calls)
			}
			if err := rb.Client.BatchCall(calls[startIndex:endIndex]); err != nil {
				logrus.Errorf("batch call is err %v", err)
				return
			}
			for _, call := range calls[startIndex:endIndex] {
				if call.Error != nil {
					logrus.Errorf("get %v from rpc node is err: %v", call.Args, call.Error)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}

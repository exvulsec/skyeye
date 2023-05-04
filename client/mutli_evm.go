package client

import (
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sirupsen/logrus"

	"go-etl/config"
)

type MultiEVMInstance struct {
	initializer func() any
	instance    any
	once        sync.Once
}

var multiEvmClient *MultiEVMInstance

func (mec *MultiEVMInstance) Instance() any {
	mec.once.Do(func() {
		mec.instance = mec.initializer()
	})
	return mec.instance
}

func initMultiEvmClient() any {
	multiClient := map[string]*ethclient.Client{}
	for chain, values := range config.Conf.HTTPServer.MultiEvmClient {
		client, err := ethclient.Dial(values.ProviderURL)
		if err != nil {
			logrus.Fatalf("failed to connect chain %s provider url %s with ethclient, err is %v", chain, values.ProviderURL, err)
		}
		logrus.Infof("connect to %s provider with ethclient is successfully", chain)
		multiClient[chain] = client
	}

	return multiClient
}

func MultiEvmClient() map[string]*ethclient.Client {
	return multiEvmClient.Instance().(map[string]*ethclient.Client)
}

func init() {
	multiEvmClient = &MultiEVMInstance{initializer: initMultiEvmClient}
}

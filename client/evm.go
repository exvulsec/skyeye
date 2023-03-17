package client

import (
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sirupsen/logrus"

	"go-etl/config"
)

var evmClient *Instance

type Instance struct {
	initializer func() any
	instance    any
	once        sync.Once
}

func (i *Instance) Instance() any {
	i.once.Do(func() {
		i.instance = i.initializer()
	})
	return i.instance
}

func initEvmClient() any {
	client, err := ethclient.Dial(config.Conf.ETLConfig.ProviderURL)
	if err != nil {
		logrus.Fatalf("failed to connect provider url %s, err is %v", config.Conf.ETLConfig.ProviderURL, err)
	}
	logrus.Infof("connect to provider is successfully")
	return client
}

func EvmClient() *ethclient.Client {
	return evmClient.Instance().(*ethclient.Client)
}

func init() {
	evmClient = &Instance{initializer: initEvmClient}
}

func NewWebSocketClient() *ethclient.Client {
	client, err := ethclient.Dial(config.Conf.ETLConfig.WebSocketURL)
	if err != nil {
		logrus.Fatalf("failed to connect provider url %s, err is %v", config.Conf.ETLConfig.ProviderURL, err)
	}
	logrus.Infof("connect to provider is successfully")
	return client
}

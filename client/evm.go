package client

import (
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sirupsen/logrus"

	"go-etl/config"
)

type EVMInstance struct {
	initializer func() any
	instance    any
	once        sync.Once
}

var evmClient *EVMInstance

func (ei *EVMInstance) Instance() any {
	ei.once.Do(func() {
		ei.instance = ei.initializer()
	})
	return ei.instance
}

func initEvmClient() any {
	client, err := ethclient.Dial(config.Conf.ETLConfig.ProviderURL)
	if err != nil {
		logrus.Fatalf("failed to connect provider url %s with ethclient, err is %v", config.Conf.ETLConfig.ProviderURL, err)
	}
	logrus.Infof("connect to provider with ethclient is successfully")
	return client
}

func EvmClient() *ethclient.Client {
	return evmClient.Instance().(*ethclient.Client)
}

func init() {
	evmClient = &EVMInstance{initializer: initEvmClient}
}

func NewWebSocketClient() *ethclient.Client {
	client, err := ethclient.Dial(config.Conf.ETLConfig.WebSocketURL)
	if err != nil {
		logrus.Fatalf("failed to connect ws provider url %s with ethclient, err is %v", config.Conf.ETLConfig.WebSocketURL, err)
	}
	logrus.Infof("connect to ws provider with ethclient is successfully")
	return client
}

package extractor

import (
	"context"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/client"
)

type blockExtractor struct {
	extractors []Extractor
}

func NewBlockExtractor(workers int) Extractor {
	return &blockExtractor{
		extractors: []Extractor{NewTransactionExtractor(workers)},
	}
}

func (be *blockExtractor) Extract(data any) {
	header, ok := data.(*types.Header)
	if ok {
		for _, e := range be.extractors {
			e.Extract(header)
		}
	}
}

func (be *blockExtractor) Run() {
	for _, t := range be.extractors {
		go func() {
			t.Run()
		}()
	}
	be.subLatestBlocks()
}

func (be *blockExtractor) subLatestBlocks() {
	headers := make(chan *types.Header)
	logrus.Info("subscribing the latest blocks...")
	sub, err := client.EvmClient().SubscribeNewHead(context.Background(), headers)
	defer sub.Unsubscribe()
	if err != nil {
		logrus.Fatalf("failed to subscribe to new blocks: %v", err)
	}
	for {
		select {
		case err = <-sub.Err():
			logrus.Fatalf("subscription block is error: %v", err)
		case header := <-headers:
			blockNumber := header.Number.Uint64()
			logrus.Infof("block: %d, is received from header", blockNumber)
			be.Extract(header)
		}
	}
}

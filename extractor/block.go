package extractor

import (
	"context"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/model"
)

type blockExtractor struct {
	extractors   []Extractor
	monitorAddrs *model.MonitorAddrs
}

func NewBlockExtractor(workers int) Extractor {
	monitorAddrs := model.MonitorAddrs{}
	if err := monitorAddrs.List(0); err != nil {
		logrus.Panicf("list monitor addr is err: %v", err)
	}
	extractors := []Extractor{
		NewTransactionExtractor(workers, &monitorAddrs),
	}
	if config.Conf.ETL.LogHashes != "" {
		extractors = append(extractors, NewLogsExtractor(workers, &monitorAddrs))
	}

	return &blockExtractor{
		extractors:   extractors,
		monitorAddrs: &monitorAddrs,
	}
}

func (be *blockExtractor) updateMonitorAddrs() {
	length := len(*be.monitorAddrs) - 1
	id := (*be.monitorAddrs)[length].ID
	newMonitorAddrs := model.MonitorAddrs{}

	if err := newMonitorAddrs.List(*id); err != nil {
		logrus.Panicf("list monitor addr is err %v", err)
	}
	*be.monitorAddrs = append(*be.monitorAddrs, newMonitorAddrs...)
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
			be.updateMonitorAddrs()
			be.Extract(header)
		}
	}
}

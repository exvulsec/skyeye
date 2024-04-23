package executor

import (
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
)

type assetExecutor struct {
	items            chan any
	executors        []Executor
	workers          int
	MonitorAddresses model.MonitorAddrs
}

func NewAssetExecutor(workers int, latestBlockNumberCh chan int64) Executor {
	monitorAddrs := model.MonitorAddrs{}
	if err := monitorAddrs.List(); err != nil {
		logrus.Panicf("list monitor addr is err %v", err)
	}

	return &assetExecutor{
		items:            make(chan any, 10),
		workers:          workers,
		MonitorAddresses: monitorAddrs,
		executors:        []Executor{NewFileExecutor(latestBlockNumberCh)},
	}
}

func (ae *assetExecutor) Name() string {
	return "Transaction"
}

func (ae *assetExecutor) GetItemsCh() chan any {
	return ae.items
}

func (ae *assetExecutor) Execute() {
	for _, e := range ae.executors {
		go e.Execute()
	}
	for range ae.workers {
		go func() {
			for item := range ae.items {
				txs, ok := item.(model.Transactions)
				if !ok || len(txs) == 0 {
					continue
				}

				blockNumber := txs[0].BlockNumber
				txs.AnalysisAssertTransfer(ae.MonitorAddresses)
				for _, e := range ae.executors {
					e.GetItemsCh() <- blockNumber
				}
			}
		}()
	}
}

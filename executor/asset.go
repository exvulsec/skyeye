package executor

import (
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
)

type assetExecutor struct {
	items            chan any
	executors        []Executor
	MonitorAddresses model.MonitorAddrs
}

func NewAssetExecutor() Executor {
	monitorAddrs := model.MonitorAddrs{}
	if err := monitorAddrs.List(); err != nil {
		logrus.Panicf("list monitor addr is err %v", err)
	}

	return &assetExecutor{
		items:            make(chan any, 10),
		MonitorAddresses: monitorAddrs,
		executors:        []Executor{NewFileExecutor()},
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
	for item := range ae.items {
		txs, ok := item.(model.Transactions)
		blockNumber := txs[0].BlockNumber
		if ok {
			txs.AnalysisAssertTransfer(ae.MonitorAddresses)
			for _, e := range ae.executors {
				e.GetItemsCh() <- blockNumber
			}
		}
	}
}

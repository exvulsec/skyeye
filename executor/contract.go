package executor

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
)

type contractExecutor struct {
	items            chan any
	executors        []Executor
	MonitorAddresses model.MonitorAddrs
}

func NewContractExecutor() Executor {
	monitorAddrs := model.MonitorAddrs{}
	if err := monitorAddrs.List(); err != nil {
		logrus.Panicf("list monitor addr is err %v", err)
	}

	return &contractExecutor{
		items:            make(chan any, 10),
		MonitorAddresses: monitorAddrs,
		executors:        []Executor{NewAssetExecutor()},
	}
}

func (ce *contractExecutor) Name() string {
	return "Transaction"
}

func (ce *contractExecutor) GetItemsCh() chan any {
	return ce.items
}

func (ce *contractExecutor) Execute() {
	for _, e := range ce.executors {
		go e.Execute()
	}
	for item := range ce.items {
		txs, ok := item.(model.Transactions)
		blockNumber := txs[0].BlockNumber
		if ok {
			contractStartTime := time.Now()
			txs.AnalysisContracts(ce.MonitorAddresses)
			logrus.Infof("processed to analysis transactions' contract on block %d, cost %.2fs",
				blockNumber, time.Since(contractStartTime).Seconds())

			for _, e := range ce.executors {
				e.GetItemsCh() <- item
			}
		}
	}
}

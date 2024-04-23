package executor

import (
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
)

type contractExecutor struct {
	items            chan any
	workers          int
	executors        []Executor
	MonitorAddresses model.MonitorAddrs
}

func NewContractExecutor(workers int, latestBlockNumberCh chan int64) Executor {
	monitorAddrs := model.MonitorAddrs{}
	if err := monitorAddrs.List(); err != nil {
		logrus.Panicf("list monitor addr is err %v", err)
	}

	return &contractExecutor{
		items:            make(chan any, 10),
		workers:          workers,
		MonitorAddresses: monitorAddrs,
		executors:        []Executor{NewAssetExecutor(workers, latestBlockNumberCh)},
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
	for range ce.workers {
		go func() {
			for item := range ce.items {
				txs, ok := item.(model.Transactions)
				if ok {
					txs.AnalysisContracts(ce.MonitorAddresses)
					for _, e := range ce.executors {
						e.GetItemsCh() <- txs
					}
				}
			}
		}()
	}
}

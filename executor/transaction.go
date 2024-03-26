package executor

import (
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
)

type transactionExecutor struct {
	items            chan any
	MonitorAddresses model.MonitorAddrs
}

func NewTransactionExecutor() Executor {
	monitorAddrs := model.MonitorAddrs{}
	if err := monitorAddrs.List(); err != nil {
		logrus.Panicf("list monitor addr is err %v", err)
	}

	return &transactionExecutor{
		items:            make(chan any, 10),
		MonitorAddresses: monitorAddrs,
	}
}

func (te *transactionExecutor) GetItemsCh() chan any {
	return te.items
}

func (te *transactionExecutor) Execute() {
	for item := range te.items {
		txs, ok := item.(model.Transactions)
		if ok {
			txs.AnalysisContracts(te.MonitorAddresses)
			txs.AnalysisAssertTransfer(te.MonitorAddresses)
		}
	}
}

package executor

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
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

func (te *transactionExecutor) Execute(workerID int) {
	for item := range te.items {
		startTime := time.Now()
		txs, ok := item.(model.Transactions)
		if ok {
			txs.AnalysisContracts(te.MonitorAddresses)
			txs.AnalysisAssertTransfer(te.MonitorAddresses)
		}
		if len(txs) > 0 {
			utils.WriteBlockNumberToFile(config.Conf.ETL.PreviousFile, txs[0].BlockNumber)
		}
		logrus.Infof("worker %d processed the transaction info cost %.2fs", workerID, time.Since(startTime).Seconds())
	}
}

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

func (te *transactionExecutor) Name() string {
	return "TransactionExecutor"
}

func (te *transactionExecutor) GetItemsCh() chan any {
	return te.items
}

func (te *transactionExecutor) Execute(workerID int) {
	for item := range te.items {
		startTime := time.Now()

		txs, ok := item.(model.Transactions)
		blockNumber := txs[0].BlockNumber
		if ok {
			txs.AnalysisContracts(te.MonitorAddresses)
			logrus.Infof("thread %d: processed to analysis transactions' contract cost %2.f", workerID, time.Since(startTime).Seconds())

			startTime = time.Now()
			txs.AnalysisAssertTransfer(te.MonitorAddresses)
			logrus.Infof("thread %d: processed to analysis transactions' asset transfer cost %2.f", workerID, time.Since(startTime).Seconds())
		}
		utils.WriteBlockNumberToFile(config.Conf.ETL.PreviousFile, blockNumber)

	}
}

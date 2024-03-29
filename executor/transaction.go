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
	return "Transaction"
}

func (te *transactionExecutor) GetItemsCh() chan any {
	return te.items
}

func (te *transactionExecutor) Execute(workerID int) {
	for item := range te.items {

		txs, ok := item.(model.Transactions)
		blockNumber := txs[0].BlockNumber
		if ok {
			contractStartTime := time.Now()
			txs.AnalysisContracts(te.MonitorAddresses)
			logrus.Infof("thread %d: processed to analysis transactions' contract on block %d, cost %.2fs",
				workerID, blockNumber, time.Since(contractStartTime).Seconds())

			assetTransferStartTime := time.Now()
			txs.AnalysisAssertTransfer(te.MonitorAddresses)
			logrus.Infof("thread %d: processed to analysis transactions' asset transfer on block %d, cost %2.fs",
				workerID, blockNumber, time.Since(assetTransferStartTime).Seconds())
		}
		utils.WriteBlockNumberToFile(config.Conf.ETL.PreviousFile, blockNumber)

	}
}

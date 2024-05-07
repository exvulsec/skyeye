package task

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type contractTask struct {
	monitorAddresses model.MonitorAddrs
}

func NewContractTask(monitorAddrs model.MonitorAddrs) Task {
	return &contractTask{
		monitorAddresses: monitorAddrs,
	}
}

func (ce *contractTask) Run(data any) any {
	txs, ok := data.(model.Transactions)
	if !ok || len(txs) == 0 {
		return nil
	}
	return ce.AnalysisContracts(txs)
}

func (ce *contractTask) AnalysisContracts(txs model.Transactions) model.Transactions {
	startTime := time.Now()
	conditionFunc := func(tx model.Transaction) bool {
		return tx.ToAddress == nil
	}

	originTxs, needAnalysisTxs := txs.MultiProcess(conditionFunc)

	if len(needAnalysisTxs) > 0 {
		needAnalysisTxs.EnrichTxs()
		for _, tx := range needAnalysisTxs {
			tx.ComposeContractAndAlert(&ce.monitorAddresses)
		}
		logrus.Infof("block: %d, analysis transactions: %d contract creation, elapsed: %s",
			needAnalysisTxs[0].BlockNumber, len(needAnalysisTxs), utils.ElapsedTime(startTime))
	}

	return append(originTxs, needAnalysisTxs...)
}

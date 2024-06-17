package task

import (
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type contractTask struct {
	monitorAddresses *model.SkyMonitorAddrs
}

func NewContractTask(monitorAddrs *model.SkyMonitorAddrs) Task {
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
		return strings.Contains(tx.Input, "60806040")
	}

	originTxs, needAnalysisTxs := txs.MultiProcess(conditionFunc)

	if len(needAnalysisTxs) > 0 {
		needAnalysisTxs.EnrichTxs()
		for index, tx := range needAnalysisTxs {
			tx.ComposeContractAndAlert(ce.monitorAddresses)
			needAnalysisTxs[index] = tx
		}
		logrus.Infof("block: %d, analysis transactions: %d contract creation, elapsed: %s",
			needAnalysisTxs[0].BlockNumber, len(needAnalysisTxs), utils.ElapsedTime(startTime))
	}

	return append(originTxs, needAnalysisTxs...)
}

package task

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
)

type contractTask struct {
	done             chan bool
	monitorAddresses model.MonitorAddrs
}

func NewContractTask(monitorAddrs model.MonitorAddrs) Task {
	return &contractTask{
		done:             make(chan bool),
		monitorAddresses: monitorAddrs,
	}
}

func (ce *contractTask) Do(data any) any {
	defer ce.setDone()
	txs, ok := data.(model.Transactions)
	if ok || len(txs) == 0 {
		return nil
	}
	return ce.AnalysisContracts(txs)
}

func (ce *contractTask) setDone() {
	ce.done <- true
}

func (ce *contractTask) Done() bool {
	return <-ce.done
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
			tx.AnalysisContract(&ce.monitorAddresses)
		}
		logrus.Infof("processed to analysis %d transactions' contract on block %d, cost %.2fs",
			len(needAnalysisTxs), needAnalysisTxs[0].BlockNumber, time.Since(startTime).Seconds())
	}

	return append(originTxs, needAnalysisTxs...)
}

package task

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
)

type assetTask struct {
	done             chan bool
	monitorAddresses model.MonitorAddrs
}

func NewAssetSubTask(monitorAddrs model.MonitorAddrs) Task {
	return &assetTask{
		done:             make(chan bool),
		monitorAddresses: monitorAddrs,
	}
}

func (at *assetTask) Do(data any) any {
	txs, ok := data.(model.Transactions)
	defer at.setDone()
	if !ok || len(txs) == 0 {
		return nil
	}
	return at.AnalysisAssetTransfer(txs)
}

func (at *assetTask) setDone() {
	at.done <- true
}

func (at *assetTask) Done() bool {
	return <-at.done
}

func (at *assetTask) AnalysisAssetTransfer(txs model.Transactions) model.Transactions {
	startTime := time.Now()
	conditionFunc := func(tx model.Transaction) bool {
		return at.monitorAddresses.Existed(*tx.ToAddress)
	}

	originTxs, needAnalysisTxs := txs.MultiProcess(conditionFunc)
	if len(needAnalysisTxs) > 0 {
		for _, tx := range needAnalysisTxs {
			tx.ComposeAssetsAndAlert()
		}
		logrus.Infof("processed to analysis %d transactions' asset transfer on block %d, cost %.2fs",
			len(needAnalysisTxs), needAnalysisTxs[0].BlockNumber, time.Since(startTime).Seconds())
	}

	return append(originTxs, needAnalysisTxs...)
}

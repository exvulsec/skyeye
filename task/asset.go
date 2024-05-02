package task

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type assetTask struct {
	done             chan bool
	monitorAddresses model.MonitorAddrs
}

func NewAssetSubTask(monitorAddrs model.MonitorAddrs) Task {
	return &assetTask{
		done:             make(chan bool, 1),
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
		logrus.Infof("block: %d, analysis transactions: %d's asset transfer, elapsed: %s",
			needAnalysisTxs[0].BlockNumber, len(needAnalysisTxs), utils.ElapsedTime(startTime))
	}

	return append(originTxs, needAnalysisTxs...)
}

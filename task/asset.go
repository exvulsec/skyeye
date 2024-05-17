package task

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type assetTask struct {
	monitorAddresses *model.MonitorAddrs
}

func NewAssetTask(monitorAddrs *model.MonitorAddrs) Task {
	return &assetTask{
		monitorAddresses: monitorAddrs,
	}
}

func (at *assetTask) Run(data any) any {
	txs, ok := data.(model.Transactions)
	if !ok || len(txs) == 0 {
		return nil
	}
	return at.AnalysisAssetTransfer(txs)
}

func (at *assetTask) AnalysisAssetTransfer(txs model.Transactions) model.Transactions {
	startTime := time.Now()
	conditionFunc := func(tx model.Transaction) bool {
		if at.monitorAddresses.Existed([]string{*tx.ToAddress}) {
			return true
		}
		if tx.MultiContracts != nil {
			return at.monitorAddresses.Existed(tx.MultiContracts)
		}
		return false
	}

	originTxs, needAnalysisTxs := txs.MultiProcess(conditionFunc)
	if len(needAnalysisTxs) > 0 {
		for _, tx := range needAnalysisTxs {
			tx.ComposeAssetsAndAlert()
		}
		logrus.Infof("block: %d, analysis transactions: %d asset transfer, elapsed: %s",
			needAnalysisTxs[0].BlockNumber, len(needAnalysisTxs), utils.ElapsedTime(startTime))
	}

	return append(originTxs, needAnalysisTxs...)
}

package task

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type transactionTask struct {
	monitorAddrs *model.MonitorAddrs
}

func NewTransactionTask(monitorAddrs *model.MonitorAddrs) Task {
	return &transactionTask{
		monitorAddrs: monitorAddrs,
	}
}

func (tt *transactionTask) Run(data any) any {
	txs, ok := data.(model.Transactions)
	if !ok || len(txs) == 0 {
		return nil
	}
	tt.MonitorTransactions(txs)
	return nil
}

func (tt *transactionTask) MonitorTransactions(txs model.Transactions) {
	startTime := time.Now()
	conditionFunc := func(tx model.Transaction) bool {
		if tx.Value.Cmp(decimal.Decimal{}) == 0 {
			return false
		}
		addrs := []string{tx.FromAddress}
		if tx.ToAddress != nil {
			addrs = append(addrs, *tx.ToAddress)
		}
		return tt.monitorAddrs.Existed(addrs)
	}

	_, needAnalysisTxs := txs.MultiProcess(conditionFunc)

	if len(needAnalysisTxs) > 0 {
		logrus.Infof("block: %d, analysis transactions: %d contract creation, elapsed: %s",
			needAnalysisTxs[0].BlockNumber, len(needAnalysisTxs), utils.ElapsedTime(startTime))
	}
	tt.Save(needAnalysisTxs)
}

func (tt *transactionTask) Save(txs model.Transactions) {
	for _, tx := range txs {
		transaction := tx.EVMTransaction
		if err := transaction.Create(); err != nil {
			logrus.Error(err)
			continue
		}

		var addr model.MonitorAddr
		if tt.monitorAddrs.Existed([]string{transaction.FromAddress}) {
			if transaction.ToAddress != nil {
				addr = model.MonitorAddr{Address: strings.ToLower(*transaction.ToAddress)}
			}
		} else {
			addr = model.MonitorAddr{Address: strings.ToLower(transaction.FromAddress)}
		}
		if err := addr.Create(config.Conf.ETL.Chain); err != nil {
			logrus.Error(err)
			continue
		}
	}
}

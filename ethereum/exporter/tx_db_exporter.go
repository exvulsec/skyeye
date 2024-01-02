package exporter

import (
	"time"

	"github.com/sirupsen/logrus"

	"go-etl/model"
)

type TransactionPostgresqlExporter struct {
	Chain string
	items chan any
}

func NewTransactionPostgresqlExporter(chain string) Exporter {
	return &TransactionPostgresqlExporter{Chain: chain, items: make(chan any, 10)}
}

func (tpe *TransactionPostgresqlExporter) GetItemsCh() chan any {
	return tpe.items
}

func (tpe *TransactionPostgresqlExporter) Run() {
	for txs := range tpe.items {
		tpe.ExportItems(txs)
	}
}

func (tpe *TransactionPostgresqlExporter) ExportItems(items any) {
	startTimestamp := time.Now()
	txs := items.(model.Transactions)

	if err := txs.CopyToDB(tpe.Chain); err != nil {
		logrus.Errorf("copy %d to db '%s.txs' is err %v", len(txs), tpe.Chain, err)
		return
	}
	logrus.Infof("copy %d txs into database cost: %.2fs", len(txs), time.Since(startTimestamp).Seconds())
}

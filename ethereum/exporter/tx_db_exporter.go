package exporter

import (
	"time"

	"github.com/sirupsen/logrus"

	"go-etl/model"
)

type TransactionPostgresqlExporter struct {
	chain   string
	workers int
	items   chan any
}

func NewTransactionPostgresqlExporter(chain string, workers int) Exporter {
	return &TransactionPostgresqlExporter{chain: chain, items: make(chan any, 10), workers: workers}
}

func (tpe *TransactionPostgresqlExporter) GetItemsCh() chan any {
	return tpe.items
}

func (tpe *TransactionPostgresqlExporter) Run() {
	for i := 0; i < tpe.workers; i++ {
		go tpe.ExportItems()
	}
}

func (tpe *TransactionPostgresqlExporter) ExportItems() {
	for txs := range tpe.items {
		tpe.exportItemsToDB(txs)
	}
}

func (tpe *TransactionPostgresqlExporter) exportItemsToDB(items any) {
	startTimestamp := time.Now()
	txs := items.(model.Transactions)

	if err := txs.CopyToDB(tpe.chain); err != nil {
		logrus.Errorf("copy %d to db '%s.txs' is err %v", len(txs), tpe.chain, err)
		return
	}
	if len(txs) > 0 {
		logrus.Infof("copy %d txs into database for block %d cost: %.2fs", len(txs), txs[0].BlockNumber, time.Since(startTimestamp).Seconds())
	}
}

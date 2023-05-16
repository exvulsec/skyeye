package exporter

import (
	"time"

	"github.com/sirupsen/logrus"

	"go-etl/config"
	"go-etl/datastore"
	"go-etl/model"
	"go-etl/utils"
)

type TransactionPostgresqlExporter struct {
	Chain string
	Nonce uint64
}

func NewTransactionExporters(chain, openAPIServer string, nonce uint64, isNastiff bool) []Exporter {
	exporters := []Exporter{}
	if !isNastiff {
		exporters = append(exporters, NewTransactionPostgresqlExporter(chain, nonce))
	} else {
		exporters = append(exporters, NewNastiffTransferExporter(chain, openAPIServer, nonce, 10))
	}
	return exporters
}

func NewTransactionPostgresqlExporter(chain string, nonce uint64) Exporter {
	return &TransactionPostgresqlExporter{Chain: chain, Nonce: nonce}
}

func (tpe *TransactionPostgresqlExporter) ExportItems(items any) {
	startTimestamp := time.Now()
	txs := items.(model.Transactions)
	txs.CreateBatchToDB(utils.ComposeTableName(
		tpe.Chain, datastore.TableTransactions),
		config.Conf.Postgresql.MaxOpenConns,
	)
	logrus.Infof("insert %d txs into database cost: %.2fs", len(txs), time.Since(startTimestamp).Seconds())
}

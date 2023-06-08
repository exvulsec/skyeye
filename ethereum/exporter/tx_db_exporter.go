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
}

func NewTransactionExporters(chain, openAPIServer string, isNastiff bool) []Exporter {
	exporters := []Exporter{}
	if !isNastiff {
		exporters = append(exporters, NewTransactionPostgresqlExporter(chain))
	} else {
		exporters = append(exporters, NewNastiffTransferExporter(chain, openAPIServer, config.Conf.ETL.ScanInterval))
	}
	return exporters
}

func NewTransactionPostgresqlExporter(chain string) Exporter {
	return &TransactionPostgresqlExporter{Chain: chain}
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

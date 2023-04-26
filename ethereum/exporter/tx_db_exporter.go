package exporter

import (
	"time"

	"github.com/sirupsen/logrus"

	"go-etl/config"
	"go-etl/model"
	"go-etl/utils"
)

var (
	TransactionAssociatedAddrs = "%s:txs_associated:addrs"
)

type TransactionPostgresqlExporter struct {
	Chain     string
	TableName string
	Nonce     uint64
}

func NewTransactionPostgresqlExporter(chain, tableName string, nonce uint64) Exporter {
	return &TransactionPostgresqlExporter{Chain: chain, TableName: tableName, Nonce: nonce}
}

func (tpe *TransactionPostgresqlExporter) ExportItems(items any) {
	startTimestamp := time.Now()
	txs := items.(model.Transactions)
	filterTXs := model.Transactions{}
	for index := range txs {
		tx := txs[index]
		if tpe.handleItem(tx) {
			filterTXs = append(filterTXs, tx)
		}
	}
	if len(filterTXs) > 0 {
		filterTXs.CreateBatchToDB(utils.ComposeTableName(
			tpe.Chain,
			tpe.TableName),
			config.Conf.Postgresql.MaxOpenConns,
		)
		logrus.Infof("insert %d txs into database cost: %.2f", len(txs), time.Since(startTimestamp).Seconds())
	}
}

func (tpe *TransactionPostgresqlExporter) handleItem(item model.Transaction) bool {
	return item.Nonce <= tpe.Nonce
}

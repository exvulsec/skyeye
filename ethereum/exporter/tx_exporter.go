package exporter

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"go-etl/config"
	"go-etl/datastore"
	"go-etl/model"
	"go-etl/utils"
)

var TransactionAssociatedAddrs = "eth_txs_associated_addrs"

type TransactionRedisExporter struct {
	Nonce int
}

func NewTransactionExporters(writeToRedis bool, nonce int) []Exporter {
	exporters := []Exporter{NewTransactionPostgresqlExporter(nonce)}
	if writeToRedis {
		exporters = append(exporters, NewTransactionRedisExporter(nonce))
	}
	return exporters
}

func NewTransactionRedisExporter(nonce int) Exporter {
	return &TransactionRedisExporter{Nonce: nonce}
}

func (tre *TransactionRedisExporter) ExportItems(items any) {
	startTimestamp := time.Now()
	for _, item := range items.(model.Transactions) {
		tre.handleItem(item)
	}
	logrus.Infof("handle %d txs to redis cost: %.2f", len(items.(model.Transactions)), time.Since(startTimestamp).Seconds())
}

func (tre *TransactionRedisExporter) handleItem(item model.Transaction) {
	if item.Nonce > tre.Nonce {
		_, err := datastore.Redis().HDel(context.Background(), TransactionAssociatedAddrs, item.FromAddress).Result()
		if err != nil {
			log.Fatalf("del %s in key %s from redis is err: %v", item.FromAddress, TransactionAssociatedAddrs, err)
		}
		return
	}
	isExist, err := datastore.Redis().HExists(context.Background(), TransactionAssociatedAddrs, item.FromAddress).Result()
	if err != nil {
		log.Fatalf("get %s in key %s from redis is err: %v", item.FromAddress, TransactionAssociatedAddrs, err)
		return
	}
	addrs := []string{}
	if isExist {
		val, err := datastore.Redis().HGet(context.Background(), TransactionAssociatedAddrs, item.FromAddress).Result()
		if err != nil {
			log.Fatalf("get %s in key %s from redis is err: %v", item.FromAddress, TransactionAssociatedAddrs, err)
			return
		}
		if val != "" {
			addrs = strings.Split(val, ",")
		}
	}
	if item.ToAddress == nil && item.ContractAddress != "" && item.Nonce <= tre.Nonce {
		addrs = append(addrs, item.ContractAddress)
		_, err = datastore.Redis().HSet(context.Background(), TransactionAssociatedAddrs, item.FromAddress, strings.Join(addrs, ",")).Result()
		if err != nil {
			log.Fatalf("set value %v to filed %s in key %s from redis is err: %v", addrs, item.FromAddress, TransactionAssociatedAddrs, err)
			return
		}
	}
}

type TransactionPostgresqlExporter struct {
	Nonce int
}

func NewTransactionPostgresqlExporter(nonce int) Exporter {
	return &TransactionPostgresqlExporter{Nonce: nonce}
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
		txs.CreateBatchToDB(utils.ComposeTableName(
			config.Conf.ETLConfig.Chain,
			datastore.TableTransactions),
			config.Conf.Postgresql.MaxOpenConns,
		)
		logrus.Infof("insert %d txs into database cost: %.2f", len(txs), time.Since(startTimestamp).Seconds())
	}

}

func (tpe *TransactionPostgresqlExporter) handleItem(item model.Transaction) bool {
	return item.Nonce <= tpe.Nonce
}

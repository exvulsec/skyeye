package exporter

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"go-etl/config"
	"go-etl/datastore"
	"go-etl/model"
	"go-etl/utils"
)

var (
	TransactionAssociatedAddrs       = "%s:txs_associated:addrs"
	TransactionContractAddressStream = "%s:contract_address:stream"
)

type TransactionRedisExporter struct {
	Chain string
	Nonce uint64
}

func NewTransactionExporters(chain, table string, nonce uint64) []Exporter {
	exporters := []Exporter{}
	if config.Conf.Postgresql.Host != "" {
		exporters = append(exporters, NewTransactionPostgresqlExporter(chain, table, nonce))
	}
	if config.Conf.RedisConfig.Addr != "" {
		exporters = append(exporters, NewTransactionRedisExporter(chain, nonce))
	}
	return exporters
}

func NewTransactionRedisExporter(chain string, nonce uint64) Exporter {
	return &TransactionRedisExporter{Chain: chain, Nonce: nonce}
}

func (tre *TransactionRedisExporter) ExportItems(items any) {
	startTimestamp := time.Now()
	for _, item := range items.(model.Transactions) {
		tre.handleItem(item)
	}
	logrus.Infof("handle %d txs to redis cost: %.2f", len(items.(model.Transactions)), time.Since(startTimestamp).Seconds())
}

func (tre *TransactionRedisExporter) handleItem(item model.Transaction) {
	key := fmt.Sprintf(TransactionAssociatedAddrs, tre.Chain)
	if item.Nonce > tre.Nonce {
		_, err := datastore.Redis().HDel(context.Background(), key, item.FromAddress).Result()
		if err != nil {
			log.Fatalf("del %s in key %s from redis is err: %v", item.FromAddress, key, err)
		}
		return
	}
	isExist, err := datastore.Redis().HExists(context.Background(), key, item.FromAddress).Result()
	if err != nil {
		log.Fatalf("get %s in key %s from redis is err: %v", item.FromAddress, TransactionAssociatedAddrs, err)
		return
	}
	addrs := []string{}
	if isExist {
		val, err := datastore.Redis().HGet(context.Background(), key, item.FromAddress).Result()
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
		_, err = datastore.Redis().HSet(context.Background(), key, item.FromAddress, strings.Join(addrs, ",")).Result()
		if err != nil {
			log.Fatalf("set value %v to filed %s in key %s from redis is err: %v", addrs, item.FromAddress, key, err)
			return
		}
		_, err = datastore.Redis().XAdd(context.Background(), &redis.XAddArgs{
			Stream: fmt.Sprintf(TransactionContractAddressStream, tre.Chain),
			ID:     "*",
			Values: map[string]any{
				item.TxHash: item.ContractAddress,
			},
		}).Result()
		if err != nil {
			log.Fatalf("send redis stream is err: %v", err)
			return
		}
	}
}

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
		txs.CreateBatchToDB(utils.ComposeTableName(
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

package exporter

import (
	"context"
	"fmt"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/datastore"
	"go-etl/model"
	"go-etl/policy"
)

var (
	TransactionAssociatedAddrs = "%s:txs_associated:addrs"
)

type TransactionRedisExporter struct {
	Chain         string
	Nonce         uint64
	OpenAPIServer string
}

func NewTransactionExporters(chain, table, openAPIServer string, nonce uint64) []Exporter {
	exporters := []Exporter{}
	if config.Conf.Postgresql.Host != "" {
		exporters = append(exporters, NewTransactionPostgresqlExporter(chain, table, nonce))
	}
	if config.Conf.Redis.Addr != "" {
		exporters = append(exporters, NewTransactionRedisExporter(chain, openAPIServer, nonce))
	}
	return exporters
}

func NewTransactionRedisExporter(chain, openAPIServer string, nonce uint64) Exporter {
	return &TransactionRedisExporter{Chain: chain, Nonce: nonce, OpenAPIServer: openAPIServer}
}

func (tre *TransactionRedisExporter) ExportItems(items any) {
	for _, item := range items.(model.Transactions) {
		if item.TxStatus != 0 {
			tre.handleItem(item)
			if item.ToAddress == nil && item.ContractAddress != "" {
				go tre.appendItemToMessageQueue(item)
			} else {
				logrus.Infof("filter the address %s by policy: create contract with tx nonce less than %d", item.ContractAddress, tre.Nonce)
			}
		}
	}
}

func (tre *TransactionRedisExporter) handleItem(item model.Transaction) {
	key := fmt.Sprintf(TransactionAssociatedAddrs, tre.Chain)
	if item.Nonce > tre.Nonce && tre.Nonce != 0 {
		_, err := datastore.Redis().HDel(context.Background(), key, item.FromAddress).Result()
		if err != nil {
			logrus.Errorf("del %s in key %s from redis is err: %v", item.FromAddress, key, err)
		}
		return
	}
	isExist, err := datastore.Redis().HExists(context.Background(), key, item.FromAddress).Result()
	if err != nil {
		logrus.Errorf("get %s in key %s from redis is err: %v", item.FromAddress, TransactionAssociatedAddrs, err)
		return
	}
	addrs := []string{}
	if isExist {
		val, err := datastore.Redis().HGet(context.Background(), key, item.FromAddress).Result()
		if err != nil {
			logrus.Errorf("get %s in key %s from redis is err: %v", item.FromAddress, TransactionAssociatedAddrs, err)
			return
		}
		if val != "" {
			addrs = strings.Split(val, ",")
		}
	}
	if item.ToAddress == nil && item.ContractAddress != "" && (tre.Nonce == 0 || item.Nonce <= tre.Nonce) {
		addrs = append(addrs, item.ContractAddress)
		_, err = datastore.Redis().HSet(context.Background(), key, item.FromAddress, strings.Join(mapset.NewSet[string](addrs...).ToSlice(), ",")).Result()
		if err != nil {
			logrus.Errorf("set value %v to filed %s in key %s from redis is err: %v", addrs, item.FromAddress, key, err)
			return
		}
	}
}

func (tre *TransactionRedisExporter) appendItemToMessageQueue(item model.Transaction) {
	code, err := client.EvmClient().CodeAt(context.Background(), common.HexToAddress(item.ContractAddress), nil)
	if err != nil {
		logrus.Errorf("get byte code for %s is err %v", item.ContractAddress, err)
		return
	}
	policyCode, err := policy.FilterContractByPolicy(tre.Chain, item.ContractAddress, item.Nonce, tre.Nonce, 10, code)
	if err != nil {
		logrus.Errorf("filter contract %s by policy is err: %s", item.ContractAddress, err)
		return
	}
	if policyCode == policy.NoAnyDenied {
		_, err = policy.SendItemToMessageQueue(tre.Chain, item.TxHash, item.ContractAddress, tre.OpenAPIServer, code, true)
		if err != nil {
			logrus.Errorf("send to txhash %s's contract %s message queue is err %v", item.TxHash, item.ContractAddress, err)
			return
		}
	}
	logrus.Infof("contract address %s is %s", item.ContractAddress, policy.DeniedMap[policyCode])
}

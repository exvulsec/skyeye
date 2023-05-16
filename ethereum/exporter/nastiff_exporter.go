package exporter

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"go-etl/datastore"
	"go-etl/model"
)

const (
	TransactionContractAddressStream = "%s:contract_address:stream"
)

type NastiffTransactionExporter struct {
	Chain         string
	Nonce         uint64
	OpenAPIServer string
	Interval      int64
}

func NewNastiffTransferExporter(chain, openserver string, nonce uint64, interval int64) Exporter {
	return &NastiffTransactionExporter{
		Chain:         chain,
		Nonce:         nonce,
		OpenAPIServer: openserver,
		Interval:      interval,
	}
}

func (nte *NastiffTransactionExporter) ExportItems(items any) {
	for _, item := range items.(model.Transactions) {
		if item.TxStatus != 0 {
			if item.ToAddress == nil && item.ContractAddress != "" {
				nt := model.NastiffTransaction{}
				nt.ConvertFromTransaction(item)
				nt.Chain = nte.Chain
				go nte.exportItem(nt)
			}
		}
	}
}

func (nte *NastiffTransactionExporter) exportItem(tx model.NastiffTransaction) {
	isFilter := nte.FilterContractByPolicies(&tx)
	if !isFilter {
		if err := nte.exportToRedis(tx); err != nil {
			logrus.Errorf("append txhash %s's contract %s to redis message queue is err %v", tx.TxHash, tx.ContractAddress, err)
			return
		}
	}
	if err := tx.Insert(true, nte.OpenAPIServer); err != nil {
		logrus.Errorf("insert txhash %s's contract %s to db is err %v", tx.TxHash, tx.ContractAddress, err)
		return
	}
	logrus.Infof("insert tx %s's contract %s to database successfully", tx.TxHash, tx.ContractAddress)
}

func (nte *NastiffTransactionExporter) FilterContractByPolicies(tx *model.NastiffTransaction) bool {
	policies := []model.FilterPolicy{
		&model.NonceFilter{ThresholdNonce: nte.Nonce},
		&model.ByteCodeFilter{},
		&model.ContractTypeFilter{},
		&model.OpenSourceFilter{Interval: nte.Interval},
	}
	policyResults := []string{}
	isFilter := false
	for _, p := range policies {
		result := "1"
		if !p.ApplyFilter(*tx) {
			result = "0"
			isFilter = true
		}
		policyResults = append(policyResults, result)
	}
	tx.Policies = strings.Join(policyResults, ",")
	return isFilter
}

func (nte *NastiffTransactionExporter) exportToRedis(tx model.NastiffTransaction) error {
	if err := tx.ComposeNastiffValues(true, nte.OpenAPIServer); err != nil {
		return err
	}
	_, err := datastore.Redis().XAdd(context.Background(), &redis.XAddArgs{
		Stream: fmt.Sprintf("%s:v2", fmt.Sprintf(TransactionContractAddressStream, nte.Chain)),
		ID:     "*",
		Values: tx.NastiffValues,
	}).Result()
	if err != nil {
		return fmt.Errorf("send values to redis stream is err: %v", err)
	}
	logrus.Infof("send tx %s's contract %s to database successfully", tx.TxHash, tx.ContractAddress)
	return nil
}

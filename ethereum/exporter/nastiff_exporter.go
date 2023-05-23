package exporter

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"go-etl/client"
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
				code, err := client.EvmClient().CodeAt(context.Background(), common.HexToAddress(nt.ContractAddress), nil)
				if err != nil {
					logrus.Errorf("get contract %s's bytecode is err %v ", nt.ContractAddress, err)
					continue
				}
				nt.ByteCode = code
				go nte.exportItem(nt)
			}
		}
	}
}

func (nte *NastiffTransactionExporter) exportItem(tx model.NastiffTransaction) {
	isFilter := nte.FilterContractByPolicies(&tx)
	if !isFilter {
		logrus.Infof("start to insert tx %s's contract %s to redis stream", tx.TxHash, tx.ContractAddress)
		if err := nte.exportToRedis(tx); err != nil {
			logrus.Errorf("append txhash %s's contract %s to redis message queue is err %v", tx.TxHash, tx.ContractAddress, err)
			return
		}
	}
	logrus.Infof("start to insert tx %s's contract %s to db", tx.TxHash, tx.ContractAddress)
	if err := tx.Insert(true, nte.OpenAPIServer); err != nil {
		logrus.Errorf("insert txhash %s's contract %s to db is err %v", tx.TxHash, tx.ContractAddress, err)
		return
	}

}

func (nte *NastiffTransactionExporter) FilterContractByPolicies(tx *model.NastiffTransaction) bool {
	policies := []model.FilterPolicy{
		&model.NonceFilter{ThresholdNonce: nte.Nonce},
		&model.ByteCodeFilter{},
		&model.ContractTypeFilter{},
		&model.OpenSourceFilter{Interval: 0},
		&model.Push4ArgsFilter{},
		&model.Push20ArgsFilter{},
	}
	policyResults := []string{}
	score := 0
	totalScore := 0
	for _, p := range policies {
		result := "1"
		if p.ApplyFilter(tx) {
			result = "0"
			score += 1
		}
		policyResults = append(policyResults, result)
		totalScore += 1
	}
	tx.Policies = strings.Join(policyResults, ",")
	tx.Score = score * 100 / totalScore
	return tx.Score != 100
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
	return nil
}

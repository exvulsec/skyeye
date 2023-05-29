package exporter

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
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
	isFilter := nte.CalcContractByPolicies(&tx)
	if !isFilter {
		logrus.Infof("start to insert tx %s's contract %s to redis stream", tx.TxHash, tx.ContractAddress)
		if err := nte.exportToRedis(tx); err != nil {
			logrus.Errorf("append txhash %s's contract %s to redis message queue is err %v", tx.TxHash, tx.ContractAddress, err)
			return
		}
	}
	logrus.Infof("start to insert tx %s's contract %s to db", tx.TxHash, tx.ContractAddress)
	if err := tx.Insert(); err != nil {
		logrus.Errorf("insert txhash %s's contract %s to db is err %v", tx.TxHash, tx.ContractAddress, err)
		return
	}

}

func (nte *NastiffTransactionExporter) CalcContractByPolicies(tx *model.NastiffTransaction) bool {
	policies := []model.PolicyCalc{
		&model.NoncePolicyCalc{ThresholdNonce: nte.Nonce},
		&model.ByteCodePolicyCalc{},
		&model.ContractTypePolicyCalc{},
		&model.OpenSourcePolicyCalc{Interval: config.Conf.ETL.ScanInterval},
		&model.Push4PolicyCalc{
			FlashLoanFuncNames: model.LoadFlashLoanFuncNames(),
		},
		&model.Push20PolicyCalc{},
		&model.FundPolicyCalc{IsNastiff: true, OpenAPIServer: nte.OpenAPIServer},
	}
	splitScores := []string{}
	totalScore := 0
	for _, p := range policies {
		score := p.Calc(tx)
		splitScores = append(splitScores, fmt.Sprintf("%d", score))
		totalScore += score
	}
	tx.SplitScores = splitScores
	tx.Score = totalScore
	return tx.Score >= config.Conf.ETL.ScoreAlertThreshold
}

func (nte *NastiffTransactionExporter) exportToRedis(tx model.NastiffTransaction) error {
	if err := tx.ComposeNastiffValues(); err != nil {
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

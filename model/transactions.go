package model

import (
	"context"

	pgx "github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/datastore"
)

type Transactions []Transaction

func (txs *Transactions) AnalysisContracts(addrs MonitorAddrs) {
	originTxs := Transactions{}
	needAnalysisTxs := Transactions{}
	for _, tx := range *txs {
		if tx.ToAddress == nil {
			needAnalysisTxs = append(needAnalysisTxs, tx)
		} else {
			originTxs = append(originTxs, tx)
		}
	}
	if len(needAnalysisTxs) > 0 {
		logrus.Infof("get %d txs is required to analysis contracts on block %d", len(needAnalysisTxs), needAnalysisTxs[0].BlockNumber)
		needAnalysisTxs.enrichTxs()
		for _, tx := range needAnalysisTxs {
			tx.AnalysisContract(&addrs)
		}
	}

	*txs = append(originTxs, needAnalysisTxs...)
}

func (txs *Transactions) AnalysisAssertTransfer(addrs MonitorAddrs) {
	originTxs := Transactions{}
	needAnalysisTxs := Transactions{}
	for _, tx := range *txs {
		if addrs.Existed(*tx.ToAddress) {
			needAnalysisTxs = append(needAnalysisTxs, tx)
		} else {
			originTxs = append(originTxs, tx)
		}
	}
	if len(needAnalysisTxs) > 0 {
		logrus.Infof("get %d txs is required to analysis asset transfer on block %d", len(needAnalysisTxs), needAnalysisTxs[0].BlockNumber)
		for _, tx := range needAnalysisTxs {
			tx.analysisAssetTransfer()
		}
	}

	*txs = append(originTxs, needAnalysisTxs...)
}

func (txs *Transactions) enrichTxs() {
	for index, tx := range *txs {
		tx.EnrichReceipt("", false)
		tx.GetTrace("", false)
		(*txs)[index] = tx
	}
}

func (txs *Transactions) CopyToDB(chain string) error {
	columns := []string{"block_timestamp", "blknum", "txhash", "txpos", "from_address", "to_address", "tx_type", "nonce", "value", "input", "contract_address", "tx_status"}
	_, err := datastore.PGX().CopyFrom(context.Background(),
		pgx.Identifier{chain, datastore.TableTransactions},
		columns,
		pgx.CopyFromSlice(len(*txs), func(i int) ([]any, error) {
			return []any{
				(*txs)[i].BlockTimestamp,
				(*txs)[i].BlockNumber,
				(*txs)[i].TxHash,
				(*txs)[i].TxPos,
				(*txs)[i].FromAddress,
				(*txs)[i].ToAddress,
				(*txs)[i].TxType,
				(*txs)[i].Nonce,
				(*txs)[i].Value,
				(*txs)[i].Input,
				(*txs)[i].ContractAddress,
				(*txs)[i].TxStatus,
			}, nil
		}),
	)
	return err
}

func (txs *Transactions) CreateBatchToDB(tableName string, batchSize int) {
	result := datastore.DB().Table(tableName).CreateInBatches(txs, batchSize)
	if result.Error != nil {
		logrus.Fatalf("insert tx into db is err %v", result.Error)
	}
}

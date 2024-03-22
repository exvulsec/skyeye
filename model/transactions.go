package model

import (
	"context"

	pgx "github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/datastore"
	"github.com/exvulsec/skyeye/utils"
)

type Transactions []Transaction

func (txs *Transactions) MonitorContractCreation() {
	originTxs := Transactions{}
	needEnrichTXs := Transactions{}
	for _, tx := range *txs {
		if tx.ToAddress == nil {
			needEnrichTXs = append(needEnrichTXs, tx)
		} else {
			originTxs = append(originTxs, tx)
		}
	}
	needEnrichTXs.enrichTxs()
	for _, tx := range needEnrichTXs {
		tx.EvaluateContractScore()
	}
	*txs = append(originTxs, needEnrichTXs...)
}

func (txs *Transactions) EvaluateContractCreation() {
}

func (txs *Transactions) MonitorAssetTransfer() {
}

func (txs *Transactions) enrichTxs() {
	for index, tx := range *txs {
		tx.enrichReceipt()
		tx.enrichTrace()
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

func (txs *Transactions) ListTransactionsWithFromAddress(tableName, address string) error {
	result := datastore.DB().Table(tableName).
		Where("from_address = ?", address).
		Order("block_timestamp asc").
		Find(txs)
	return result.Error
}

func (txs *Transactions) FilterAssociatedAddrs(chain, fromAddr string, filterAddrs []string) error {
	txList := Transactions{}
	tableName := utils.ComposeTableName(chain, datastore.TableAssociatedTxs)
	if err := txList.ListTransactionsWithFromAddress(tableName, fromAddr); err != nil {
		return err
	}
	for index := range txList {
		if txList[index].filterAddrs(filterAddrs) {
			*txs = append(*txs, txList[index])
		}
	}
	return nil
}

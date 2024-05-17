package model

import (
	"context"
	"sync"

	pgx "github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
)

type Transactions []Transaction

func (txs *Transactions) MultiProcess(condition func(tx Transaction) bool) (Transactions, Transactions) {
	originTxs := Transactions{}
	needAnalysisTxs := Transactions{}
	mutex := sync.RWMutex{}
	workers := make(chan int, 3)
	wg := sync.WaitGroup{}

	cleanFunc := func() {
		wg.Done()
		<-workers
	}

	for _, tx := range *txs {
		workers <- 1
		wg.Add(1)
		go func() {
			defer cleanFunc()
			mutex.Lock()
			if condition(tx) {
				needAnalysisTxs = append(needAnalysisTxs, tx)
			} else {
				originTxs = append(originTxs, tx)
			}
			mutex.Unlock()
		}()
	}
	wg.Wait()
	return originTxs, needAnalysisTxs
}

func (txs *Transactions) EnrichTxs() {
	for index, tx := range *txs {
		tx.GetTrace(config.Conf.ETL.Chain)
		tx.EnrichReceipt(config.Conf.ETL.Chain)
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

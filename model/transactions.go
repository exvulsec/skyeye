package model

import (
	"context"
	"sync"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/datastore"
)

type Transactions []Transaction

func (txs *Transactions) multiProcess(condition func(tx Transaction) bool) (Transactions, Transactions) {
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

func (txs *Transactions) AnalysisContracts(addrs MonitorAddrs) {
	startTime := time.Now()
	conditionFunc := func(tx Transaction) bool {
		return tx.ToAddress == nil
	}

	originTxs, needAnalysisTxs := txs.multiProcess(conditionFunc)

	if len(needAnalysisTxs) > 0 {
		needAnalysisTxs.enrichTxs()
		for _, tx := range needAnalysisTxs {
			tx.AnalysisContract(&addrs)
		}
		logrus.Infof("processed to analysis %d transactions' contract on block %d, cost %.2fs",
			len(needAnalysisTxs), needAnalysisTxs[0].BlockNumber, time.Since(startTime).Seconds())
	}

	*txs = append(originTxs, needAnalysisTxs...)
}

func (txs *Transactions) AnalysisAssertTransfer(addrs MonitorAddrs) {
	startTime := time.Now()
	conditionFunc := func(tx Transaction) bool {
		return addrs.Existed(*tx.ToAddress)
	}

	originTxs, needAnalysisTxs := txs.multiProcess(conditionFunc)
	if len(needAnalysisTxs) > 0 {
		for _, tx := range needAnalysisTxs {
			tx.analysisAssetTransfer()
		}
		logrus.Infof("processed to analysis %d transactions' asset transfer on block %d, cost %.2fs",
			len(needAnalysisTxs), needAnalysisTxs[0].BlockNumber, time.Since(startTime).Seconds())
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

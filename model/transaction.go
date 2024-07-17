package model

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/datastore"
	"github.com/exvulsec/skyeye/utils"
)

type Transaction struct {
	EVMTransaction
	ContractAddress string            `json:"contract_address" gorm:"column:contract_address"`
	Input           string            `json:"input" gorm:"column:input"`
	Receipt         *types.Receipt    `json:"receipt" gorm:"-"`
	Trace           *TransactionTrace `json:"trace" gorm:"-"`
	SplitScores     string            `json:"-" gorm:"-"`
	MultiContracts  []string          `json:"-" gorm:"-"`
	IsConstructor   bool              `json:"-" gorm:"-"`
}

func (tx *Transaction) ConvertFromBlock(transaction *types.Transaction, index int64) {
	fromAddr, err := types.Sender(types.LatestSignerForChainID(transaction.ChainId()), transaction)
	if err != nil {
		logrus.Fatalf("get from address is err: %v", err)
	}

	tx.TxHash = strings.ToLower(transaction.Hash().String())
	tx.FromAddress = strings.ToLower(fromAddr.String())
	if transaction.To() != nil {
		toAddr := strings.ToLower(transaction.To().String())
		tx.ToAddress = &toAddr
	}
	tx.TxPos = index
	tx.Nonce = transaction.Nonce()
	tx.TxType = transaction.Type()
	tx.Input = fmt.Sprintf("0x%s", hex.EncodeToString(transaction.Data()))
	tx.Value = decimal.NewFromBigInt(transaction.Value(), 0)
}

func (tx *Transaction) GetLatestRecord(chain string) error {
	tableName := utils.ComposeTableName(chain, datastore.TableTransactions)
	return datastore.DB().Table(tableName).Order("id DESC").Limit(1).Find(tx).Error
}

func (tx *Transaction) GetReceipt(chain string) {
	receipt, ok := utils.Retry(func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return client.MultiEvmClient()[chain].TransactionReceipt(ctx, common.HexToHash(tx.TxHash))
	}).(*types.Receipt)
	if ok {
		tx.Receipt = receipt
	}
}

func (tx *Transaction) EnrichReceipt(chain string) {
	tx.GetReceipt(chain)
	if tx.Receipt != nil {
		tx.TxPos = int64(tx.Receipt.TransactionIndex)
		tx.TxStatus = int64(tx.Receipt.Status)
	}
}

func (tx *Transaction) filterAddrs(addrs []string) bool {
	for _, addr := range addrs {
		if strings.EqualFold(tx.ContractAddress, addr) {
			return true
		}
	}
	return false
}

func (tx *Transaction) GetTrace(chain string) {
	var trace *TransactionTrace
	trace, ok := utils.Retry(func() (any, error) {
		ctxWithTimeOut, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
		defer cancel()
		err := client.MultiEvmClient()[chain].Client().CallContext(ctxWithTimeOut, &trace,
			"debug_traceTransaction",
			common.HexToHash(tx.TxHash),
			map[string]string{
				"tracer": "callTracer",
			})

		return trace, err
	}).(*TransactionTrace)
	if ok {
		tx.Trace = trace
	}
}

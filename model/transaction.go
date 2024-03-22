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
	BlockTimestamp  int64             `json:"block_timestamp" gorm:"column:block_timestamp"`
	BlockNumber     int64             `json:"block_number" gorm:"column:blknum"`
	TxHash          string            `json:"txhash" gorm:"column:txhash"`
	TxPos           int64             `json:"txpos" gorm:"column:txpos"`
	FromAddress     string            `json:"from_address" gorm:"column:from_address"`
	ToAddress       *string           `json:"to_address" gorm:"column:to_address"`
	TxType          uint8             `json:"tx_type" gorm:"column:tx_type"`
	ContractAddress string            `json:"contract_address" gorm:"column:contract_address"`
	Input           string            `json:"input" gorm:"column:input"`
	Value           decimal.Decimal   `json:"value" gorm:"column:value"`
	TxStatus        int64             `json:"tx_status" gorm:"column:tx_status"`
	Nonce           uint64            `json:"nonce" gorm:"column:nonce"`
	Receipt         *types.Receipt    `json:"receipt" gorm:"-"`
	Trace           *TransactionTrace `json:"trace" gorm:"-"`
}

func (tx *Transaction) ConvertFromBlock(transaction *types.Transaction) {
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
	tx.Nonce = transaction.Nonce()
	tx.TxType = transaction.Type()
	tx.Input = fmt.Sprintf("0x%s", hex.EncodeToString(transaction.Data()))
	tx.Value = decimal.NewFromBigInt(transaction.Value(), 0)
}

func (tx *Transaction) GetLatestRecord(chain string) error {
	tableName := utils.ComposeTableName(chain, datastore.TableTransactions)
	return datastore.DB().Table(tableName).Order("id DESC").Limit(1).Find(tx).Error
}

func (tx *Transaction) getReceipt() {
	fn := func(element any) (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		txHash, ok := element.(common.Hash)
		if !ok {
			return nil, fmt.Errorf("element type is required  tx hash")
		}
		return client.EvmClient().TransactionReceipt(ctx, txHash)
	}
	receipt := utils.Retry(6, common.HexToHash(tx.TxHash), fn).(*types.Receipt)
	if receipt == nil {
		logrus.Infof("get receipt with txhash %s failed, drop it", tx.TxHash)
		return
	}
	tx.Receipt = receipt
}

func (tx *Transaction) enrichReceipt() {
	tx.getReceipt()
	if tx.Receipt != nil {
		if tx.ContractAddress != utils.ZeroAddress {
			tx.ContractAddress = strings.ToLower(tx.Receipt.ContractAddress.String())
		}
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

func (tx *Transaction) enrichTrace() {
	var trace *TransactionTrace
	fn := func(element any) (any, error) {
		ctxWithTimeOut, cancel := context.WithTimeout(context.TODO(), 20*time.Second)
		defer cancel()
		err := client.EvmClient().Client().CallContext(ctxWithTimeOut, &trace,
			"debug_traceTransaction",
			common.HexToHash(tx.TxHash),
			map[string]string{
				"tracer": "callTracer",
			})
		return trace, err
	}
	trace = utils.Retry(5, trace, fn).(*TransactionTrace)

	if trace == nil {
		logrus.Infof("get trace with txhash %s failed, drop it", tx.TxHash)
	}
	tx.Trace = trace
}

func (tx *Transaction) EvaluateContractScore() {
	policies := []PolicyCalc{
		&MultiContractCalc{},
		&FundPolicyCalc{NeedFund: true},
		&NoncePolicyCalc{},
	}
	skyTx := SkyEyeTransaction{}
	skyTx.ConvertFromTransaction(*tx)
	for _, p := range policies {
		if p.Filter(&skyTx) {
			return
		}
		score := p.Calc(&skyTx)
		skyTx.Scores = append(skyTx.Scores, fmt.Sprintf("%s: %d", p.Name(), score))
		skyTx.Score += score
	}
	for _, contract := range skyTx.MultiContracts {
		newSkyTx := SkyEyeTransaction{
			BlockTimestamp:  skyTx.BlockTimestamp,
			BlockNumber:     skyTx.BlockNumber,
			TxHash:          skyTx.TxHash,
			TxPos:           skyTx.TxPos,
			FromAddress:     skyTx.FromAddress,
			ContractAddress: contract,
			Nonce:           skyTx.Nonce,
			Score:           skyTx.Score,
			Scores:          skyTx.Scores,
			Fund:            skyTx.Fund,
		}
		newSkyTx.Evaluate()
		newSkyTx.Alert()
	}
}

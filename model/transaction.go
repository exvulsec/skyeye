package model

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/datastore"
	"go-etl/utils"
)

type Transactions []Transaction

type Transaction struct {
	BlockTimestamp  int64           `json:"block_timestamp" gorm:"column:block_timestamp"`
	BlockNumber     int64           `json:"block_number" gorm:"column:blknum"`
	TxHash          string          `json:"txhash" gorm:"column:tx_hash"`
	TxPos           int64           `json:"txpos" gorm:"column:tx_pos"`
	FromAddress     string          `json:"from_address" gorm:"column:from_address"`
	ToAddress       *string         `json:"to_address" gorm:"column:to_address"`
	TxType          uint8           `json:"tx_type" gorm:"column:tx_type"`
	ContractAddress string          `json:"contract_address" gorm:"column:contract_address"`
	Input           string          `json:"input" gorm:"column:input"`
	Value           decimal.Decimal `json:"value" gorm:"column:value"`
	TxStatus        int64           `json:"tx_status" gorm:"column:tx_status"`
	Nonce           int             `json:"nonce" gorm:"-"`
	Gas
}

type Gas struct {
	GasBase  decimal.Decimal `json:"gas_base" gorm:"column:gas_base"`
	GasCap   decimal.Decimal `json:"gas_cap" gorm:"column:gas_max"`
	GasTips  decimal.Decimal `json:"gas_tips" gorm:"column:gas_tips"`
	GasLimit int64           `json:"gas_limit" gorm:"column:gas_limit"`
	GasPrice decimal.Decimal `json:"gas_price" gorm:"column:gas_price"`
	GasUsed  int64           `json:"gas_used" gorm:"column:gas_used"`
	GasFee   decimal.Decimal `json:"gas_fee" gorm:"column:gas_fee"`
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
	tx.TxType = transaction.Type()
	tx.Input = fmt.Sprintf("0x%s", hex.EncodeToString(transaction.Data()))
	tx.Value = decimal.NewFromBigInt(transaction.Value(), 0)
	tx.GasCap = decimal.NewFromBigInt(transaction.GasPrice(), 0)
	tx.GasTips = decimal.NewFromBigInt(transaction.GasTipCap(), 0)
	tx.GasLimit = int64(transaction.Gas())
}

func (txs *Transactions) EnrichReceipts(batchSize, workers int) {
	calls := []rpc.BatchElem{}
	for index := range *txs {
		calls = append(calls, rpc.BatchElem{
			Method: utils.RPCNameEthGetTransactionReceipt,
			Args:   []any{common.HexToHash((*txs)[index].TxHash)},
			Result: &types.Receipt{},
		})
	}
	client.MultiCall(calls, batchSize, workers)
	for index := range *txs {
		receipt, _ := calls[index].Result.(*types.Receipt)
		(*txs)[index].enrichReceipt(*receipt)
	}
}

func (tx *Transaction) enrichReceipt(receipt types.Receipt) {
	tx.GasUsed = int64(receipt.GasUsed)
	tx.GasPrice = decimal.NewFromBigInt(receipt.EffectiveGasPrice, 0)
	tx.GasFee = decimal.NewFromInt(tx.GasUsed).Mul(tx.GasPrice)
	if tx.ContractAddress != utils.ZeroAddress {
		tx.ContractAddress = strings.ToLower(receipt.ContractAddress.String())
	}
	tx.TxPos = int64(receipt.TransactionIndex)
	tx.TxStatus = int64(receipt.Status)
}

func (txs *Transactions) CreateBatchToDB(tableName string, worker int) {
	result := datastore.DB().Table(tableName).CreateInBatches(txs, worker)
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
	tableName := utils.ComposeTableName(chain, datastore.TableContractCreationTxs)
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

func (tx *Transaction) filterAddrs(addrs []string) bool {
	for _, addr := range addrs {
		if tx.ContractAddress == strings.ToLower(addr) {
			return true
		}
	}
	return false
}

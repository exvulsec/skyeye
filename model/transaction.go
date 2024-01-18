package model

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jackc/pgx/v5"
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
	TxHash          string          `json:"txhash" gorm:"column:txhash"`
	TxPos           int64           `json:"txpos" gorm:"column:txpos"`
	FromAddress     string          `json:"from_address" gorm:"column:from_address"`
	ToAddress       *string         `json:"to_address" gorm:"column:to_address"`
	TxType          uint8           `json:"tx_type" gorm:"column:tx_type"`
	ContractAddress string          `json:"contract_address" gorm:"column:contract_address"`
	Input           string          `json:"input" gorm:"column:input"`
	Value           decimal.Decimal `json:"value" gorm:"column:value"`
	TxStatus        int64           `json:"tx_status" gorm:"column:tx_status"`
	Nonce           uint64          `json:"nonce" gorm:"column:nonce"`
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
	tx.Nonce = transaction.Nonce()
	tx.TxType = transaction.Type()
	tx.Input = fmt.Sprintf("0x%s", hex.EncodeToString(transaction.Data()))
	tx.Value = decimal.NewFromBigInt(transaction.Value(), 0)
	tx.GasCap = decimal.NewFromBigInt(transaction.GasPrice(), 0)
	tx.GasTips = decimal.NewFromBigInt(transaction.GasTipCap(), 0)
	tx.GasLimit = int64(transaction.Gas())
}

func getReceiptWithTimeOut(txHash common.Hash) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return client.EvmClient().TransactionReceipt(ctx, txHash)
}

func getReceipt(txHash common.Hash) *types.Receipt {
	for i := 0; i < 6; i++ {
		receipt, err := getReceiptWithTimeOut(txHash)
		if err != nil && !utils.IsRetriableError(err) {
			logrus.Errorf("get receipt for tx %s is err %v", txHash, err)
			break
		}
		if receipt != nil {
			return receipt
		}
		time.Sleep(1 * time.Second)
		logrus.Infof("retry %d times to get tx's receipt %s", i+1, txHash)
	}
	logrus.Infof("get receipt with txhash %s failed, drop it", txHash)
	return nil
}

func (txs *Transactions) EnrichReceipts() {
	for index := range *txs {
		txHash := common.HexToHash((*txs)[index].TxHash)
		receipt := getReceipt(txHash)
		if receipt != nil {
			(*txs)[index].enrichReceipt(*receipt)
		}
	}
}

func (tx *Transaction) enrichReceipt(receipt types.Receipt) {
	tx.GasUsed = int64(receipt.GasUsed)
	if receipt.EffectiveGasPrice != nil {
		tx.GasPrice = decimal.NewFromBigInt(receipt.EffectiveGasPrice, 0)
		tx.GasFee = decimal.NewFromInt(tx.GasUsed).Mul(tx.GasPrice)
	}
	contractAddress := strings.ToLower(receipt.ContractAddress.String())
	if tx.ContractAddress != utils.ZeroAddress {
		tx.ToAddress = &contractAddress
		multiContract := []string{contractAddress}
		txTraces := GetTransactionTrace(tx.TxHash)
		for index := range txTraces {
			txTrace := txTraces[index]
			if txTrace.ContractAddress != "" && len(txTrace.TraceAddress) != 0 {
				multiContract = append(multiContract, strings.ToLower(txTrace.ContractAddress))
			}
		}
		tx.ContractAddress = strings.Join(multiContract, ",")
	}
	tx.TxPos = int64(receipt.TransactionIndex)
	tx.TxStatus = int64(receipt.Status)
}

func (txs *Transactions) CopyToDB(chain string) error {
	columns := []string{"block_timestamp", "blknum", "txhash", "txpos", "from_address", "to_address", "tx_type", "nonce", "value", "input", "contract_address", "tx_status"}
	_, err := datastore.PGX().CopyFrom(context.Background(),
		pgx.Identifier{chain, datastore.TableTransactions},
		columns,
		pgx.CopyFromSlice(len(*txs), func(i int) ([]any, error) {
			return []any{(*txs)[i].BlockTimestamp,
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

func (tx *Transaction) filterAddrs(addrs []string) bool {
	for _, addr := range addrs {
		if strings.EqualFold(tx.ContractAddress, addr) {
			return true
		}
	}
	return false
}

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
	"github.com/exvulsec/skyeye/config"
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
	MultiContracts  []string          `json:"-" gorm:"-"`
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

func (tx *Transaction) getReceipt(chain string) {
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
	tx.getReceipt(chain)
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

func (tx *Transaction) ComposeContractAndAlert(addrs *MonitorAddrs) {
	policies := []PolicyCalc{
		&FundPolicyCalc{Chain: config.Conf.ETL.Chain, NeedFund: true},
		&NoncePolicyCalc{},
	}
	skyTx := SkyEyeTransaction{}
	skyTx.ConvertFromTransaction(*tx)
	contracts, skip := tx.Trace.ListContracts()
	if skip {
		return
	}
	tx.MultiContracts = contracts
	skyTx.MultiContracts = contracts
	skyTx.MultiContractString = strings.Join(skyTx.MultiContracts, ",")

	for _, p := range policies {
		if p.Filter(&skyTx) {
			return
		}
		score := p.Calc(&skyTx)
		skyTx.Scores = append(skyTx.Scores, fmt.Sprintf("%s: %d", p.Name(), score))
		skyTx.Score += score
	}
	for _, contract := range skyTx.MultiContracts {
		contractTX := SkyEyeTransaction{
			Chain:               skyTx.Chain,
			BlockTimestamp:      skyTx.BlockTimestamp,
			BlockNumber:         skyTx.BlockNumber,
			TxHash:              skyTx.TxHash,
			TxPos:               skyTx.TxPos,
			FromAddress:         skyTx.FromAddress,
			ContractAddress:     contract,
			Nonce:               skyTx.Nonce,
			Score:               skyTx.Score,
			Scores:              skyTx.Scores,
			Fund:                skyTx.Fund,
			MonitorAddrs:        addrs,
			MultiContractString: skyTx.MultiContractString,
		}
		contractTX.Analysis(config.Conf.ETL.Chain)
		if !contractTX.Skip {
			contractTX.alert()
		}
	}
}

func (tx *Transaction) ComposeAssetsAndAlert() {
	assets := Assets{
		BlockNumber:    tx.BlockNumber,
		BlockTimestamp: tx.BlockTimestamp,
		TxHash:         tx.TxHash,
		Items:          []Asset{},
	}
	if tx.Trace == nil {
		tx.GetTrace(config.Conf.ETL.Chain)
	}
	if tx.Receipt == nil {
		tx.getReceipt(config.Conf.ETL.Chain)
	}
	if tx.Receipt != nil && tx.Trace != nil {
		skyTx := SkyEyeTransaction{Input: tx.Input}
		if tx.ToAddress != nil {
			assets.ToAddress = *tx.ToAddress
		}

		if tx.MultiContracts == nil {
			if err := skyTx.GetInfoByContract(config.Conf.ETL.Chain, *tx.ToAddress); err != nil {
				logrus.Errorf("get skyeye tx info is err %v", err)
			}
			if skyTx.ID == nil {
				return
			}
			if skyTx.MultiContractString != "" {
				skyTx.MultiContracts = strings.Split(skyTx.MultiContractString, ",")
			}
		} else {
			skyTx.MultiContracts = tx.MultiContracts
		}
		assetTransfers := AssetTransfers{}

		assetTransfers.compose(tx.Receipt.Logs, *tx.Trace)
		if err := assets.AnalysisAssetTransfers(assetTransfers); err != nil {
			logrus.Errorf("analysis asset transfer is err: %v", err)
			return
		}
		if len(assets.Items) > 0 {
			assets.alert(skyTx)
		}
	}
}

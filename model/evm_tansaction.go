package model

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
)

var (
	FromAddressSource = "from_address"
	ToAddressSource   = "to_address"
)

type EVMTransaction struct {
	BlockTimestamp int64           `json:"block_timestamp" gorm:"column:block_timestamp"`
	BlockNumber    int64           `json:"block_number" gorm:"column:blknum"`
	TxHash         string          `json:"txhash" gorm:"column:txhash"`
	TxPos          int64           `json:"txpos" gorm:"column:txpos"`
	FromAddress    string          `json:"from_address" gorm:"column:from_address"`
	ToAddress      *string         `json:"to_address" gorm:"column:to_address"`
	TxType         uint8           `json:"tx_type" gorm:"column:tx_type"`
	Value          decimal.Decimal `json:"value" gorm:"column:value"`
	TxStatus       int64           `json:"tx_status" gorm:"column:tx_status"`
	Nonce          uint64          `json:"nonce" gorm:"column:nonce"`
}

func (et *EVMTransaction) TableName() string {
	return fmt.Sprintf("%s.%s", config.Conf.ETL.Chain, datastore.TableTransactions)
}

func (et *EVMTransaction) Create() error {
	return datastore.DB().Table(et.TableName()).Create(et).Error
}

type EVMTransactions []EVMTransaction

func (ets *EVMTransactions) TableName(chain string) string {
	if chain == "" {
		chain = config.Conf.ETL.Chain
	}

	return fmt.Sprintf("%s.%s", chain, datastore.TableTransactions)
}

func (ets *EVMTransactions) GetByAddress(source, chain, address string) error {
	engine := datastore.DB().Table(ets.TableName(chain))
	switch source {
	case FromAddressSource:
		engine = engine.Where("from_address = ?", address)
	case ToAddressSource:
		engine = engine.Where("to_address = ?", address)
	}
	return engine.Find(ets).Error
}

func (ets *EVMTransactions) ComposeNodeEdges() []NodeEdge {
	nodeEdges := []NodeEdge{}
	for _, evmTX := range *ets {
		nodeEdges = append(nodeEdges, NodeEdge{
			Timestamp:   evmTX.BlockTimestamp,
			TxHash:      evmTX.TxHash,
			Token:       EVMPlatformCurrency,
			Value:       evmTX.Value,
			FromAddress: evmTX.FromAddress,
			ToAddress:   *evmTX.ToAddress,
		})
	}
	return nodeEdges
}

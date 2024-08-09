package model

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
)

type TokenTransfer struct {
	BlockTimestamp int64           `json:"block_timestamp" gorm:"column:block_timestamp"`
	BlockNumber    int64           `json:"block_number" gorm:"column:blknum"`
	TxHash         string          `json:"txhash" gorm:"column:txhash"`
	TxPos          int64           `json:"txpos" gorm:"column:txpos"`
	Logpos         int64           `json:"logpos" gorm:"column:logpos"`
	TokenAddress   string          `json:"token_address" gorm:"column:token_address"`
	FromAddress    string          `json:"from_address" gorm:"column:from_address"`
	ToAddress      string          `json:"to_address" gorm:"column:to_address"`
	Value          decimal.Decimal `json:"value" gorm:"column:value"`
}

func (tt *TokenTransfer) TableName() string {
	return fmt.Sprintf("%s.%s", config.Conf.ETL.Chain, datastore.TableTokenTransfers)
}

func (tt *TokenTransfer) Create() error {
	return datastore.DB().Table(tt.TableName()).Create(tt).Error
}

func (tt *TokenTransfer) DecodeFromEvent(event Event, log types.Log) error {
	at := AssetTransfer{}
	at.DecodeEvent(event, log)
	tt.FromAddress = at.From
	tt.ToAddress = at.To

	if at.Value.Cmp(decimal.Decimal{}) == 0 {
		return nil
	}

	tt.BlockNumber = int64(log.BlockNumber)
	block, err := client.MultiEvmClient()[config.Conf.ETL.Chain].BlockByNumber(context.Background(), big.NewInt(tt.BlockNumber))
	if err != nil {
		return err
	}
	tt.BlockTimestamp = int64(block.Time())

	tt.Value = at.Value
	tt.TxHash = log.TxHash.Hex()
	tt.TokenAddress = at.Address

	tt.Logpos = int64(log.Index)
	tt.TxPos = int64(log.TxIndex)
	if err := tt.Create(); err != nil {
		return fmt.Errorf("create token transfer is err: %v", err)
	}
	var addr MonitorAddr
	if err := addr.Create(config.Conf.ETL.Chain); err != nil {
		return fmt.Errorf("create monitor address %s is err: %v", tt.ToAddress, err)
	}
	return nil
}

type TokenTransfers []TokenTransfer

func (tts *TokenTransfers) TableName(chain string) string {
	if chain == "" {
		chain = config.Conf.ETL.Chain
	}
	return fmt.Sprintf("%s.%s", chain, datastore.TableTokenTransfers)
}

func (tts *TokenTransfers) GetByAddress(source, chain, address string) error {
	engine := datastore.DB().Table(tts.TableName(chain))
	switch source {
	case FromAddressSource:
		engine = engine.Where("from_address = ?", address)
	case ToAddressSource:
		engine = engine.Where("to_address = ?", address)
	}
	return engine.Find(tts).Error
}

func (tts *TokenTransfers) ComposeNodes() []NodeEdge {
	nodeEdges := []NodeEdge{}
	for _, tt := range *tts {
		nodeEdges = append(nodeEdges, NodeEdge{
			Timestamp:   tt.BlockTimestamp,
			TxHash:      tt.TxHash,
			Token:       tt.TokenAddress,
			Value:       tt.Value,
			FromAddress: tt.FromAddress,
			ToAddress:   tt.ToAddress,
		})
	}
	return nodeEdges
}

package model

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"

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

func (tt *TokenTransfer) DecodeFromEvent(event Event, log types.Log, addrs MonitorAddrs) error {
	tt.FromAddress = convertAddress(event["from"].(common.Address).String())
	tt.ToAddress = convertAddress(event["to"].(common.Address).String())
	if !addrs.Existed([]string{tt.FromAddress, tt.ToAddress}) {
		return nil
	}
	tt.Value = decimal.NewFromBigInt(event["value"].(*big.Int), 0)
	if tt.Value.Cmp(decimal.Decimal{}) == 0 {
		return nil
	}

	tt.TokenAddress = strings.ToLower(log.Address.String())
	tt.BlockNumber = int64(log.BlockNumber)
	tt.Logpos = int64(log.Index)
	tt.TxPos = int64(log.TxIndex)
	if err := tt.Create(); err != nil {
		return fmt.Errorf("create token transfer is err: %v", err)
	}
	var addr MonitorAddr
	if addrs.Existed([]string{tt.FromAddress}) {
		addr = MonitorAddr{Address: strings.ToLower(tt.ToAddress)}
	} else {
		addr = MonitorAddr{Address: strings.ToLower(tt.FromAddress)}
	}
	if err := addr.Create(config.Conf.ETL.Chain); err != nil {
		return fmt.Errorf("create monitor address %s is err: %v", tt.ToAddress, err)
	}
	return nil
}

package model

import (
	"go-etl/datastore"

	"github.com/shopspring/decimal"
)

type TokenPrice struct {
	Name      string          `json:"name" gorm:"column:name"`
	Symbol    string          `json:"symbol" gorm:"column:symbol"`
	Address   string          `json:"address" gorm:"column:address"`
	Decimals  int             `json:"decimals" gorm:"column:decimals"`
	Timestamp int             `json:"timestamp" gorm:"column:timestamp"`
	Price     decimal.Decimal `json:"price" gorm:"column:price"`
}

type TokenPrices []TokenPrice

func (tp *TokenPrice) GetTokenPrice(tableName string, addr string) error {
	return datastore.DB().
		Table(tableName).
		Where("address=?", addr).
		Order("timestamp").
		Limit(1).
		Find(tp).Error
}

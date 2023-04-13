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

func (tps *TokenPrices) GetTokenPrices(tableName string, addrs []string) error {
	result := datastore.DB().
		Table(tableName).
		Where("address in (?)", addrs).
		Find(tps)
	return result.Error
}

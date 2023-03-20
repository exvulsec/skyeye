package model

import (
	"github.com/shopspring/decimal"

	"go-etl/database"
)

type Token struct {
	Address     string          `json:"address" gorm:"column:address"`
	Name        string          `json:"name" gorm:"column:name"`
	Symbol      string          `json:"symbol" gorm:"column:symbol"`
	TotalSupply decimal.Decimal `json:"total_supply" gorm:"column:total_supply"`
	IsErc20     bool            `json:"is_erc20" gorm:"column:is_erc20"`
	IsErc721    bool            `json:"is_erc721" gorm:"column:is_erc721"`
	IsErc1155   bool            `json:"is_erc1155" gorm:"column:is_erc1155"`
}

func (t *Token) GetToken(tableName, address string) error {
	result := database.DB().
		Table(tableName).
		Where("address = ?", address).
		Find(t)
	return result.Error
}

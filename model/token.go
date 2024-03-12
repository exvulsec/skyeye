package model

import (
	"go-etl/datastore"
	"go-etl/utils"
)

type Token struct {
	Address  string `json:"address" gorm:"column:address"`
	Name     string `json:"name" gorm:"column:name"`
	Symbol   string `json:"symbol" gorm:"column:symbol"`
	Decimals int64  `json:"decimals" gorm:"column:decimals"`
}

func (t *Token) GetToken(chain, address string) error {
	return datastore.DB().
		Table(utils.ComposeTableName(chain, datastore.TableTokens)).
		Where("address = ?", address).
		Find(t).Error
}

func (t *Token) Create(chain string) error {
	return datastore.DB().
		Table(utils.ComposeTableName(chain, datastore.TableTokens)).
		Create(t).Error
}

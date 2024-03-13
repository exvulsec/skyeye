package model

import (
	"github.com/shopspring/decimal"

	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
	"go-etl/client"
	"go-etl/datastore"
	"go-etl/model/erc20"
	"go-etl/utils"
)

type Token struct {
	ID       *int64          `json:"id" gorm:"column:id"`
	Address  string          `json:"address" gorm:"column:address"`
	Name     string          `json:"name" gorm:"column:name"`
	Symbol   string          `json:"symbol" gorm:"column:symbol"`
	Decimals int64           `json:"decimals" gorm:"column:decimals"`
	Value    decimal.Decimal `json:"value" gorm:"column:-"`
	ValueUSD decimal.Decimal `json:"value_usd" gorm:"column:-"`
}

func (t *Token) IsExisted(chain, address string) bool {
	err := datastore.DB().
		Table(utils.ComposeTableName(chain, datastore.TableTokens)).
		Where("address = ?", address).
		Find(t).Error
	if err != nil {
		logrus.Panic(err)
		return false
	}
	return t.ID != nil
}

func (t *Token) Create(chain string) error {
	return datastore.DB().
		Table(utils.ComposeTableName(chain, datastore.TableTokens)).
		Create(t).Error
}

func (t *Token) GetMetadataOnChain(chain, address string) {
	token, err := erc20.NewErc20(common.HexToAddress(address), client.MultiEvmClient()[chain])
	if err != nil {
		logrus.Panicf("failed to instantiate a token contract %s: %v", address, err)
		return
	}
	name, err := token.Name(nil)
	symbol, err := token.Symbol(nil)
	decimals, err := token.Decimals(nil)

	t.Address = address
	t.Name = name
	t.Symbol = symbol
	t.Decimals = int64(decimals)
	if err = t.Create(chain); err != nil {
		logrus.Panicf("create token %s to db is err %v", address, err)
		return
	}
}

func (t *Token) GetSymbol() string {
	if t.Symbol == "" {
		return t.Address
	}
	return t.Symbol
}

func (t *Token) GetValueWithDecimals(value decimal.Decimal) decimal.Decimal {
	pow := decimal.NewFromInt(10).Pow(decimal.NewFromInt(t.Decimals))
	return value.DivRound(pow, 20)
}

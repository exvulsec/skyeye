package model

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
)

type Approval struct {
	ID          *int64          `json:"id" gorm:"column:id"`
	BlockNumber int64           `json:"blknum" gorm:"column:blknum"`
	Token       string          `json:"token" gorm:"column:token"`
	Owner       string          `json:"owner" gorm:"column:owner"`
	Spender     string          `json:"spender" gorm:"column:spender"`
	Amount      decimal.Decimal `json:"amount" gorm:"column:amount"`
}

func GetApprovalPreviousBlockNumber(chain string, blockNumber *uint64) error {
	tableName := fmt.Sprintf("%s.%s", chain, datastore.TableApprovals)
	return datastore.DB().Table(tableName).Select("COALESCE(MAX(blknum), 0)").Find(blockNumber).Error
}

func (a *Approval) DecodeFromEvent(event Event, log types.Log) {
	if token, ok := event["token"]; ok {
		a.Token = strings.ToLower(token.(common.Address).String())
	} else {
		a.Token = strings.ToLower(log.Address.String())
	}
	a.Owner = strings.ToLower(event["owner"].(common.Address).String())
	a.Spender = strings.ToLower(event["spender"].(common.Address).String())

	var amount decimal.Decimal
	if event.mapKeyExist("value") {
		amount = decimal.NewFromBigInt(event["value"].(*big.Int), 0)
	} else if event.mapKeyExist("approved") {
		if approved, ok := event["approved"].(bool); ok && approved {
			amount = decimal.NewFromInt(2).Pow(decimal.NewFromInt32(256)).Sub(decimal.NewFromInt(1))
		} else {
			amount = decimal.NewFromInt(0)
		}
	}

	if err := a.Upsert(config.Conf.ETL.Chain, amount, log.BlockNumber); err != nil {
		logrus.Errorf("upsert the owner %s, spender %s, token %s's approval data to db is err: %v", a.Owner, a.Spender, a.Token, err)
	}
}

func (a *Approval) Upsert(chain string, amount decimal.Decimal, blockNumber uint64) error {
	existed, err := a.isExisted(chain)
	if err != nil {
		return err
	}
	a.Amount = amount
	a.BlockNumber = int64(blockNumber)
	if existed {
		return a.Update(chain)
	} else {
		return a.Create(chain)
	}
}

func (a *Approval) Update(chain string) error {
	return datastore.DB().Table(a.TableName(chain)).Updates(a).Error
}

func (a *Approval) Create(chain string) error {
	return datastore.DB().Table(a.TableName(chain)).Create(a).Error
}

func (a *Approval) Delete(chain string) error {
	return datastore.DB().Table(a.TableName(chain)).Delete(a).Where("id = ?", a.ID).Error
}

func (a *Approval) TableName(chain string) string {
	return fmt.Sprintf("%s.%s", chain, datastore.TableApprovals)
}

func (a *Approval) isExisted(chain string) (bool, error) {
	if err := datastore.DB().Table(a.TableName(chain)).
		Where("owner = ?", a.Owner).
		Where("spender = ?", a.Spender).
		Where("token = ?", a.Token).Find(a).Error; err != nil {
		return false, err
	}
	return a.ID != nil, nil
}

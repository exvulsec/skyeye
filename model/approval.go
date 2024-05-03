package model

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/exvulsec/skyeye/datastore"
)

var tableName = fmt.Sprintf("%s.%s", datastore.SchemaPublic, datastore.TableApprovals)

type Approval struct {
	ID          int64           `json:"id" gorm:"column:id"`
	Chain       string          `json:"chain" gorm:"column:chain"`
	BlockNumber int64           `json:"blknum" gorm:"column:blknum"`
	Token       string          `json:"token" gorm:"column:token"`
	Owner       string          `json:"owner" gorm:"column:owner"`
	Spender     string          `json:"spender" gorm:"column:spender"`
	Amount      decimal.Decimal `json:"amount" gorm:"column:amount"`
}

func GetApprovalPreviousBlockNumber(chain string, blockNumber *uint64) error {
	return datastore.DB().Table(tableName).Select("max(blknum) as blknum").Where("chain = ?", chain).Find(blockNumber).Error
}

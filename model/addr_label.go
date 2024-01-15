package model

import (
	"fmt"
	"strings"

	"go-etl/datastore"
	"go-etl/utils"
)

const (
	TornadoCash = "Tornado.Cash"
	ChangeNow   = "ChangeNOW"
)

type AddressLabel struct {
	Chain   string `json:"chain" gorm:"column:chain"`
	Address string `json:"address" gorm:"column:address"`
	Label   string `json:"label" gorm:"column:label"`
}

func (al *AddressLabel) GetLabel(chain, address string) error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableLabels)
	err := datastore.DB().Table(tableName).
		Where("chain = ?", chain).
		Where("address = ?", address).
		Limit(1).
		Find(al).Error
	if err != nil {
		return fmt.Errorf("get %s's label from db is err: %v", address, err)
	}

	if al.Label != "" {
		return nil
	}
	al.Address = address
	al.Chain = chain

	return nil
}

func (al *AddressLabel) IsTornadoCashAddress() bool {
	return strings.HasPrefix(al.Label, TornadoCash)
}

func (al *AddressLabel) Create() error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableLabels)
	return datastore.DB().Table(tableName).Create(al).Error
}

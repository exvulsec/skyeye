package model

import (
	"fmt"

	"go-etl/datastore"
)

type AddressLabel struct {
	Address string `json:"address" gorm:"column:address"`
	Name    string `json:"name" gorm:"column:name"`
	Labels  string ` json:"labels" gorm:"column:labels"`
}

func (al *AddressLabel) GetLabels(chain, address string) error {
	tableName := fmt.Sprintf("%s.%s", chain, datastore.TableLabels)
	return datastore.DB().Table(tableName).
		Where("address = ?", address).
		Limit(1).
		Find(al).Error
}

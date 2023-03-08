package model

import (
	"fmt"

	"openapi/database"
)

type AddressLabel struct {
	Address string `json:"address" gorm:"column:address"`
	Name    string `json:"name" gorm:"column:name"`
	Labels  string ` json:"labels" gorm:"column:labels"`
}

func (al *AddressLabel) GetLabels(chain, address string) error {
	tableName := fmt.Sprintf("%s.labels", chain)
	return database.DB().Table(tableName).
		Where("address = ?", address).First(al).Error
}

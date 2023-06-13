package model

import (
	"github.com/sirupsen/logrus"

	"go-etl/datastore"
	"go-etl/utils"
)

type MonitorAddr struct {
	ID          int    `json:"id" gorm:"column:id"`
	Chain       string `json:"chain" gorm:"column:chain"`
	Address     string `json:"address" gorm:"column:address"`
	Description string `json:"description" gorm:"column:description"`
}

func (ma *MonitorAddr) Create() error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableMonitorAddrs)
	if ma.Exist() {
		return nil
	}
	return datastore.DB().Table(tableName).Create(ma).Error
}

func (ma *MonitorAddr) Get(chain, address string) error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableMonitorAddrs)
	return datastore.DB().Table(tableName).
		Where("chain = ?", chain).
		Where("address = ?", address).
		Find(ma).Error
}

func (ma *MonitorAddr) Exist() bool {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableMonitorAddrs)
	err := datastore.DB().Table(tableName).
		Where("chain = ?", ma.Chain).
		Where("address = ?", ma.Address).
		Find(ma).Error
	if err != nil {
		logrus.Errorf("get chain %s address %s is err %v", ma.Chain, ma.Address, err)
		return false
	}
	return ma.ID != 0
}

func (ma *MonitorAddr) Delete() error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableMonitorAddrs)
	return datastore.DB().Table(tableName).
		Where("chain = ?", ma.Chain).
		Where("address = ?", ma.Address).
		Delete(nil).Error
}

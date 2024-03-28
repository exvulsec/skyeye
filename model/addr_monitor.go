package model

import (
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
	"github.com/exvulsec/skyeye/utils"
)

type MonitorAddr struct {
	ID          int    `json:"id" gorm:"column:id"`
	Chain       string `json:"chain" gorm:"column:chain"`
	Address     string `json:"address" gorm:"column:address"`
	Description string `json:"description" gorm:"column:description"`
}

type MonitorAddrs []MonitorAddr

var monitorAddrTableName = utils.ComposeTableName(datastore.SchemaPublic, datastore.TableMonitorAddrs)

func (ma *MonitorAddr) Create() error {
	if ma.Exist() {
		return nil
	}
	return datastore.DB().Table(monitorAddrTableName).Create(ma).Error
}

func (ma *MonitorAddr) Get(chain, address string) error {
	return datastore.DB().Table(monitorAddrTableName).
		Where("chain = ?", chain).
		Where("address = ?", address).
		Find(ma).Error
}

func (ma *MonitorAddr) Exist() bool {
	err := datastore.DB().Table(monitorAddrTableName).
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
	return datastore.DB().Table(monitorAddrTableName).
		Where("chain = ?", ma.Chain).
		Where("address = ?", ma.Address).
		Delete(nil).Error
}

func (mas *MonitorAddrs) List() error {
	return datastore.DB().Table(monitorAddrTableName).
		Where("chain = ?", config.Conf.ETL.Chain).
		Find(mas).Error
}

func (mas *MonitorAddrs) Existed(addr string) bool {
	for _, monitorAddr := range *mas {
		if strings.EqualFold(monitorAddr.Address, addr) {
			return true
		}
	}
	return false
}

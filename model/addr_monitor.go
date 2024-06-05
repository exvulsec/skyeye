package model

import (
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
	"github.com/exvulsec/skyeye/utils"
)

type MonitorAddr struct {
	ID          int        `json:"id" gorm:"column:id"`
	Chain       string     `json:"chain" gorm:"column:chain"`
	Address     string     `json:"address" gorm:"column:address"`
	Description string     `json:"description" gorm:"column:description"`
	CreatedTime *time.Time `json:"-" gorm:"column:created_at"`
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
		Order("id asc").
		Find(mas).Error
}

func (mas *MonitorAddrs) Existed(addrs []string) bool {
	for _, addr := range addrs {
		for _, monitorAddr := range *mas {
			if strings.EqualFold(monitorAddr.Address, addr) &&
				time.Now().Sub(*monitorAddr.CreatedTime) <= time.Duration(config.Conf.ETL.WatchingDuration)*time.Minute {
				return true
			}
		}
	}
	return false
}

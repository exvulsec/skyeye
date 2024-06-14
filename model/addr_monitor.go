package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
)

type MonitorAddr struct {
	ID          *int64     `json:"id" gorm:"column:id"`
	Chain       string     `json:"chain" gorm:"column:chain"`
	Address     string     `json:"address" gorm:"column:address"`
	Description string     `json:"description" gorm:"column:description"`
	CreatedTime *time.Time `json:"-" gorm:"column:created_at"`
	isSkyEye    bool       `gorm:"-"`
}

type MonitorAddrs []MonitorAddr

func (ma *MonitorAddr) TableName() string {
	if ma.isSkyEye {
		return fmt.Sprintf("%s.%s", datastore.SchemaPublic, datastore.TableMonitorAddrs)
	}
	return fmt.Sprintf("%s.%s", ma.Chain, datastore.TableMonitorAddrs)
}

func (mas *MonitorAddrs) TableName() string {
	return fmt.Sprintf("%s.%s", datastore.SchemaPublic, datastore.TableMonitorAddrs)
}

func (ma *MonitorAddr) Create() error {
	if err := ma.Get(); err != nil {
		return err
	}
	if ma.ID != nil {
		return nil
	}
	return datastore.DB().Table(ma.TableName()).Create(ma).Error
}

func (ma *MonitorAddr) Get() error {
	return datastore.DB().Table(ma.TableName()).
		Where("address = ?", ma.Address).
		Find(ma).Error
}

func (ma *MonitorAddr) Delete() error {
	return datastore.DB().Table(ma.TableName()).
		Where("address = ?", ma.Address).
		Delete(nil).Error
}

func (mas *MonitorAddrs) List() error {
	return datastore.DB().Table(mas.TableName()).
		Where("chain = ?", config.Conf.ETL.Chain).
		Order("id asc").
		Find(mas).Error
}

func (mas *MonitorAddrs) Existed(addrs []string) bool {
	now := time.Now().Unix()
	for _, addr := range addrs {
		for _, m := range *mas {
			duration := now - m.CreatedTime.Unix()
			if strings.EqualFold(m.Address, addr) &&
				duration <= config.Conf.ETL.WatchingDuration*int64(time.Minute.Seconds()) {
				return true
			}
		}
	}
	return false
}

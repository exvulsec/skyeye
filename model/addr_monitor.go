package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
)

type SkyEyeMonitorAddress struct {
	Chain string `json:"chain" gorm:"column:chain"`
	MonitorAddr
}

func (sma *SkyEyeMonitorAddress) TableName() string {
	return fmt.Sprintf("%s.%s", datastore.SchemaPublic, datastore.TableMonitorAddrs)
}

func (ma *MonitorAddr) TableName(chain string) string {
	return fmt.Sprintf("%s.%s", chain, datastore.TableMonitorAddrs)
}

func (sma *SkyEyeMonitorAddress) Create() error {
	if err := sma.Get(); err != nil {
		return err
	}
	if sma.ID != nil {
		return nil
	}
	return datastore.DB().Table(sma.TableName()).Create(sma).Error
}

func (sma *SkyEyeMonitorAddress) Get() error {
	return datastore.DB().Table(sma.TableName()).
		Where("address = ?", sma.Address).
		Where("chain = ?", sma.Chain).
		Find(sma).Error
}

func (sma *SkyEyeMonitorAddress) Delete() error {
	return datastore.DB().Table(sma.TableName()).
		Where("address = ?", sma.Address).
		Where("chain = ?", sma.Chain).
		Delete(nil).Error
}

type SkyMonitorAddrs []SkyEyeMonitorAddress

func (smas *SkyMonitorAddrs) TableName() string {
	return fmt.Sprintf("%s.%s", datastore.SchemaPublic, datastore.TableMonitorAddrs)
}

func (smas *SkyMonitorAddrs) List() error {
	return datastore.DB().Table(smas.TableName()).
		Where("chain = ?", config.Conf.ETL.Chain).
		Order("id asc").
		Find(smas).Error
}

func (smas *SkyMonitorAddrs) Existed(addrs []string) bool {
	now := time.Now().Unix()
	for _, addr := range addrs {
		for _, m := range *smas {
			duration := now - m.CreatedAt.Unix()
			if strings.EqualFold(m.Address, addr) &&
				duration <= config.Conf.ETL.WatchingDuration*int64(time.Minute.Seconds()) {
				return true
			}
		}
	}
	return false
}

type MonitorAddr struct {
	ID          *int64     `json:"id" gorm:"column:id"`
	Address     string     `json:"address" gorm:"column:address"`
	Description string     `json:"description" gorm:"column:description"`
	CreatedAt   *time.Time `json:"created_at" gorm:"column:created_at"`
}

func (ma *MonitorAddr) Create(chain string) error {
	if err := ma.Get(chain); err != nil {
		return err
	}
	if ma.ID != nil {
		return nil
	}
	return datastore.DB().Table(ma.TableName(chain)).Create(ma).Error
}

func (ma *MonitorAddr) Get(chain string) error {
	return datastore.DB().Table(ma.TableName(chain)).
		Where("address = ?", ma.Address).
		Find(ma).Error
}

func (ma *MonitorAddr) Delete(chain string) error {
	return datastore.DB().Table(ma.TableName(chain)).
		Where("address = ?", ma.Address).
		Delete(nil).Error
}

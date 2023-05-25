package model

import (
	"go-etl/datastore"
	"go-etl/utils"
)

type MonitorAddr struct {
	ID      int    `json:"id" gorm:"column:id"`
	Chain   string `json:"chain" gorm:"column:chain"`
	Address string `json:"address" gorm:"column:address"`
}

func (ma *MonitorAddr) Create() error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableMonitorAddrs)
	return datastore.DB().Table(tableName).Create(ma).Error
}

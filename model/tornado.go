package model

import (
	"github.com/sirupsen/logrus"

	"go-etl/datastore"
	"go-etl/utils"
)

type Tornado struct {
	ID      int64 `gorm:"column:id"`
	Chain   int64 `gorm:"column:chain"`
	Address int64 `gorm:"column:address"`
}

func (t *Tornado) GetTornadoAddress(chain, address string) error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableTornadoLogs)
	return datastore.DB().Table(tableName).
		Where("chain = ?", chain).
		Where("receipt = ?", address).
		Find(t).Error
}

func (t *Tornado) Exist(chain, address string) bool {
	if err := t.GetTornadoAddress(chain, address); err != nil {
		logrus.Errorf("get tornado log from address %s is err %v", address, err)
		return false
	}
	return t.ID != 0
}

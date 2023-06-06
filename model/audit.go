package model

import (
	"time"

	"go-etl/datastore"
	"go-etl/utils"
)

type CMCAudit struct {
	CoinID      *int      `json:"coinId" gorm:"column:cmc_coinid"`
	Name        string    `json:"coinName" gorm:"column:name"`
	Symbol      string    `json:"symbol" gorm:"column:symbol"`
	Address     string    `json:"contractAddress" gorm:"column:address"`
	Chain       string    `json:"contractPlatform" gorm:"column:chain"`
	Auditor     string    `json:"auditor" gorm:"-"`
	AuditStatus int       `json:"auditStatus" gorm:"column:status"`
	AuditTime   time.Time `json:"auditTime" gorm:"column:audit_time"`
	ReportURL   string    `json:"reportUrl" gorm:"column:report_url"`
}

type CMCAudits []CMCAudit

func (a *CMCAudits) GetCMCAudits() error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableAuditContracts)
	return datastore.DB().Table(tableName).Order("cmc_coinid IS NULL, cmc_coinid desc, audit_time desc").Find(a).Error
}

package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-etl/model"
	"go-etl/utils"
)

const (
	AuditStatusNotAudited = iota
	AuditStatusAuditing
	AuditStatusAudited
)

type AuditController struct{}

func (ac *AuditController) Routers(routers gin.IRouter) {
	routers.GET("/audit_reports", ac.GetAuditReports)
	routers.POST("/audit_reports", ac.AddAuditReports)
}

func (ac *AuditController) AddAuditReports(c *gin.Context) {
	auditReport := model.AuditReport{}

	if err := c.BindJSON(&auditReport); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("unmarhsal body is err %v", err)})
		return
	}
	auditReport.Chain = utils.GetSupportChain(auditReport.Chain)
	auditReport.Address = strings.ToLower(auditReport.Address)

	if auditReport.AuditStatus == AuditStatusAudited {
		auditReport.AuditTime = time.Now()
	}

	if err := auditReport.Create(); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: auditReport})
}

func (ac *AuditController) GetAuditReports(c *gin.Context) {
	audits := model.AuditReports{}
	if err := audits.GetAuditReports(); err != nil {
		c.JSON(http.StatusOK, model.Message{
			Code: http.StatusInternalServerError,
			Msg:  fmt.Errorf("get audit is err: %v", err).Error(),
		})
		return
	}

	for index := range audits {
		audits[index].Auditor = "Numen Cyber Labs"
	}

	c.JSON(http.StatusOK, model.Message{
		Code: http.StatusOK,
		Data: audits,
	})
}

package controller

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"go-etl/model"
)

type AuditController struct{}

func (ac *AuditController) Routers(routers gin.IRouter) {
	routers.GET("/audit_reports", ac.GetAuditReports)
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

package controller

import (
	"github.com/gin-gonic/gin"
)

type CMCController struct {
	AuditController AuditController
}

func (cc *CMCController) Routers(routers gin.IRouter) {
	routers.GET("/cmc", cc.AuditController.GetAuditReports)
}

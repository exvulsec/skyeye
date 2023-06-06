package controller

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"go-etl/model"
)

type CMCController struct{}

func (cc *CMCController) Routers(routers gin.IRouter) {
	routers.GET("/cmc", cc.GetAudit)
}

func (cc *CMCController) GetAudit(c *gin.Context) {
	audits := model.CMCAudits{}
	if err := audits.GetCMCAudits(); err != nil {
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

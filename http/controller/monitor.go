package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type MonitorController struct {
}

func (mc *MonitorController) Routers(routers gin.IRouter) {
	routers.POST("/monitoring", mc.AppendMonitorAddress)
	routers.DELETE("/monitoring/:address", mc.DeleteMonitorAddress)
	routers.GET("/monitoring/:address", mc.GetMonitorAddress)
}

func (mc *MonitorController) GetMonitorAddress(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Query("address"))
	monitorAddr := model.MonitorAddr{}
	if err := monitorAddr.Get(chain, address); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: monitorAddr})
}

func (mc *MonitorController) AppendMonitorAddress(c *gin.Context) {
	monitorAddr := model.MonitorAddr{}
	if err := c.BindJSON(&monitorAddr); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("unmarhsal body is err %v", err)})
		return
	}
	monitorAddr.Chain = utils.GetSupportChain(monitorAddr.Chain)
	monitorAddr.Address = strings.ToLower(monitorAddr.Address)

	if err := monitorAddr.Create(); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: monitorAddr})
}

func (mc *MonitorController) DeleteMonitorAddress(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Param("address"))
	monitor := model.MonitorAddr{Chain: chain, Address: address}
	if err := monitor.Delete(); err != nil {
		c.JSON(http.StatusOK, model.Message{
			Code: http.StatusInternalServerError,
			Msg:  fmt.Sprintf("delete chain %s address %s from db is err: %v", chain, address, err),
		})
		return
	}

	c.JSON(http.StatusOK, model.Message{
		Code: http.StatusOK,
		Msg:  fmt.Sprintf("delete chain %s address %s from db successfully", chain, address),
	})
}

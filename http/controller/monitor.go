package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type MonitorController struct{}

func (mc *MonitorController) Routers(routers gin.IRouter) {
	routers.POST("/monitoring", mc.AppendMonitorAddress)
	routers.DELETE("/monitoring/:address", mc.DeleteMonitorAddress)
	routers.GET("/monitoring/:address", mc.GetMonitorAddress)
}

func (mc *MonitorController) GetMonitorAddress(c *gin.Context) {
	monitorAddr := model.MonitorAddr{
		Chain:   utils.GetSupportChain(c.Query(utils.ChainKey)),
		Address: strings.ToLower(c.Param("address")),
	}
	if monitorAddr.Chain == utils.ChainEmpty {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: "required chain name, but got empty"})
		return
	}
	if err := monitorAddr.Get(); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: monitorAddr})
}

func (mc *MonitorController) AppendMonitorAddress(c *gin.Context) {
	monitorAddr := model.MonitorAddr{
		Chain:   utils.GetSupportChain(c.Query(utils.ChainKey)),
		Address: strings.ToLower(c.Param("address")),
	}
	if monitorAddr.Chain == utils.ChainEmpty {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: "required chain name, but got empty"})
		return
	}
	if err := monitorAddr.Create(); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: monitorAddr})
}

func (mc *MonitorController) DeleteMonitorAddress(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Param("address"))
	monitorAddr := model.MonitorAddr{Chain: chain, Address: address}
	if monitorAddr.Chain == utils.ChainEmpty {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: "required chain name, but got empty"})
		return
	}

	if err := monitorAddr.Delete(); err != nil {
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

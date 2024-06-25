package controller

import (
	"errors"
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
	routers.GET("/monitoring/:address/txs", mc.GetMonitorAddress)
}

func (mc *MonitorController) GetQuery(c *gin.Context) (string, model.MonitorAddr, error) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Query("address"))
	if chain == utils.ChainEmpty {
		return chain, model.MonitorAddr{}, errors.New("required chain name, but got empty")
	}
	if address == "" {
		return chain, model.MonitorAddr{}, errors.New("required address, but got empty")
	}
	monitorAddr := model.MonitorAddr{
		Address: address,
	}
	return chain, monitorAddr, nil
}

func (mc *MonitorController) GetMonitorAddress(c *gin.Context) {
	chain, monitorAddr, err := mc.GetQuery(c)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: err.Error()})
		return
	}
	if err := monitorAddr.Get(chain); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: monitorAddr})
}

func (mc *MonitorController) GetMonitorAddressTXs(c *gin.Context) {
	chain, monitorAddr, err := mc.GetQuery(c)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: err.Error()})
		return
	}

	txs := model.EVMTransaction{}
}

func (mc *MonitorController) AppendMonitorAddress(c *gin.Context) {
	chain, monitorAddr, err := mc.GetQuery(c)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: err.Error()})
		return
	}

	if err := monitorAddr.Create(chain); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: monitorAddr})
}

func (mc *MonitorController) DeleteMonitorAddress(c *gin.Context) {
	chain, monitorAddr, err := mc.GetQuery(c)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: err.Error()})
		return
	}
	if err := monitorAddr.Delete(chain); err != nil {
		c.JSON(http.StatusOK, model.Message{
			Code: http.StatusInternalServerError,
			Msg:  fmt.Sprintf("delete chain %s address %s from db is err: %v", chain, monitorAddr.Address, err),
		})
		return
	}

	c.JSON(http.StatusOK, model.Message{
		Code: http.StatusOK,
		Msg:  fmt.Sprintf("delete chain %s address %s from db successfully", chain, monitorAddr.Address),
	})
}

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
	routers.GET("/monitoring/:address/graph", mc.GetMonitorGraph)
	routers.GET("/monitoring/:address/txs", mc.GetMonitorAddress)
}

func (mc *MonitorController) getAddress(c *gin.Context) (string, error) {
	address := strings.ToLower(c.Query("address"))
	if address != "" {
		return address, nil
	}
	address = strings.ToLower(c.Param("address"))
	if address != "" {
		return address, nil
	}
	return "", errors.New("required address, but got empty")
}

func (mc *MonitorController) GetQuery(c *gin.Context) (string, model.MonitorAddr, error) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Query("address"))
	if chain == utils.ChainEmpty {
		return chain, model.MonitorAddr{}, errors.New("required chain name, but got empty")
	}
	address, err := mc.getAddress(c)
	if err != nil {
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

	c.JSON(http.StatusOK, model.Message{Code: 0, Data: monitorAddr})
}

func (mc *MonitorController) GetMonitorGraph(c *gin.Context) {
	chain, monitorAddr, err := mc.GetQuery(c)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: err.Error()})
		return
	}

	graph, err := model.NewGraph(chain, monitorAddr.Address)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Message{Code: 0, Data: graph})
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

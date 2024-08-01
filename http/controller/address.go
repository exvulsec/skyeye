package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type AddressController struct {
	fpc model.FundPolicyCalc
}

func (ac *AddressController) Routers(routers gin.IRouter) {
	api := routers.Group("/address")
	{
		api.GET("/:address/fund", ac.GetFund)
		api.GET("/:address/tx_graph", ac.GetTransactionGraph)
	}
}

func (ac *AddressController) GetFund(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))

	address := strings.ToLower(c.Param("address"))
	txResp, err := ac.fpc.SearchFund(chain, address)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: txResp})
}

func (ac *AddressController) GetTransactionGraph(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))

	address := strings.ToLower(c.Param("address"))
	graph, err := ac.fpc.GetAddressTransactionGraph(chain, address)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
	}

	c.JSON(http.StatusOK, model.Message{Code: 0, Data: graph})
}

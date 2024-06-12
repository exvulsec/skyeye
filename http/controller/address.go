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
	}
}

func (ac *AddressController) GetFund(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))

	address := strings.ToLower(c.Param("address"))
	txResp, err := ac.fpc.GetFund(chain, address)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: txResp})
}

package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-etl/model"
	"go-etl/model/policy"
	"go-etl/utils"
)

type AddressController struct {
	fpc policy.FundPolicyCalc
}

func (ac *AddressController) Routers(routers gin.IRouter) {
	api := routers.Group("/address")
	{
		api.GET("/:address/labels", ac.GetAddressLabel)
		api.GET("/:address/associated", ac.AssociatedByAddress)
		api.GET("/:address/fund", ac.GetFund)
	}
}

func (ac *AddressController) GetAddressLabel(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Param("address"))
	label := model.AddressLabel{}
	if err := label.GetLabel(chain, address); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: label})
}

func (ac *AddressController) AssociatedByAddress(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Param("address"))
	filterAddrs := strings.Split(c.Query("filter_addrs"), ",")
	txs := model.Transactions{}
	if len(filterAddrs) > 0 {
		if err := txs.FilterAssociatedAddrs(chain, address, filterAddrs); err != nil {
			c.JSON(
				http.StatusOK,
				model.Message{
					Code: http.StatusInternalServerError,
					Msg:  fmt.Sprintf("list the contract creation is err %v", err),
				})
			return
		}
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: txs})
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

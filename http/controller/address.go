package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-etl/model"
)

type AddressController struct{}

func (ac *AddressController) Routers(routers gin.IRouter) {
	api := routers.Group("/address")
	{
		api.GET("/:address/labels", ac.FindLabelByAddress)
		api.GET("/:address/associated", ac.AssociatedByAddress)
	}
}

func (ac *AddressController) FindLabelByAddress(c *gin.Context) {
	chain := c.Query("chain")
	address := strings.ToLower(c.Param("address"))
	if address == "" {
		c.JSON(
			http.StatusOK,
			model.Message{
				Code: http.StatusBadRequest,
				Msg:  "the address argument in the path is empty",
			})
		return
	}
	addrLabel := model.AddressLabel{}
	if err := addrLabel.GetLabels(chain, address); err != nil {
		c.JSON(
			http.StatusOK,
			model.Message{
				Code: http.StatusInternalServerError,
				Msg:  fmt.Sprintf("get address %s is err: %v", address, err),
			})
		return
	}
	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: addrLabel})
}

func (ac *AddressController) AssociatedByAddress(c *gin.Context) {
	chain := c.Query("chain")
	address := strings.ToLower(c.Param("address"))
	if address == "" {
		c.JSON(
			http.StatusOK,
			model.Message{
				Code: http.StatusBadRequest,
				Msg:  "the address argument in the path is empty",
			})
		return
	}

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

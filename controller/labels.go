package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"openapi/model"
)

type AddressController struct{}

func (ac *AddressController) Routers(routers gin.IRouter) {
	api := routers.Group("/address")
	{
		api.GET("/:address/labels", ac.FindLabelByAddress)
	}
}

func (ac *AddressController) FindLabelByAddress(c *gin.Context) {
	chain := c.Query("chain")
	address := strings.ToLower(c.Param("address"))
	if address == "" {
		c.JSON(
			http.StatusOK,
			gin.H{
				"code": http.StatusBadRequest,
				"msg":  "the address argument in the path is empty",
			})
		return
	}
	addrLabel := model.AddressLabel{}
	if err := addrLabel.GetLabels(chain, address); err != nil {
		c.JSON(
			http.StatusOK,
			gin.H{
				"code": http.StatusInternalServerError,
				"msg":  fmt.Sprintf("get address %s is err: %v", address, err),
			})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": addrLabel})
}

package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-etl/datastore"
	"go-etl/model"
	"go-etl/utils"
)

type TokenController struct{}

func (tc *TokenController) Routers(routers gin.IRouter) {
	api := routers.Group("/tokens")
	{
		api.GET("/:address", tc.GetTokenInfo)
	}
	routers.GET("/token_prices", tc.GetTokenPrices)
}

func (tc *TokenController) GetTokenInfo(c *gin.Context) {
	chain := utils.GetChainFromQuery(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Param("address"))

	token := model.Token{}
	if err := token.GetToken(utils.ComposeTableName(chain, datastore.TableTokens), address); err != nil {
		c.JSON(
			http.StatusOK,
			model.Message{
				Code: http.StatusInternalServerError,
				Msg:  fmt.Sprintf("get address %s is err: %v", address, err),
			})
		return
	}
	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: token})
}

func (tc *TokenController) GetTokenPrices(c *gin.Context) {
	chain := utils.GetChainFromQuery(c.Query(utils.ChainKey))
	addressString := c.Query("address")
	addrs := strings.Split(addressString, ",")
	if len(addrs) == 0 {
		c.JSON(
			http.StatusOK,
			model.Message{
				Code: http.StatusBadRequest,
				Msg:  "required token address, but got empty",
			})
		return
	}
	for index := range addrs {
		addrs[index] = strings.ToLower(addrs[index])
	}
	tokenPrices := model.TokenPrices{}
	if err := tokenPrices.GetTokenPrices(utils.ComposeTableName(chain, datastore.TableTokenPrices), addrs); err != nil {
		c.JSON(
			http.StatusOK,
			model.Message{
				Code: http.StatusInternalServerError,
				Msg:  fmt.Sprintf("get address %s is err: %v", addressString, err),
			})
		return
	}
	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: tokenPrices})
}

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

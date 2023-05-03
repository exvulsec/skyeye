package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-etl/config"
	"go-etl/model"
)

const APIKEY = "apikey"

func CheckAPIKEY() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Query(APIKEY) != config.Conf.HTTPServer.APIKey {
			c.AbortWithStatusJSON(http.StatusOK, model.Message{
				Code: http.StatusUnauthorized,
				Msg:  "invalid api key",
			})
		}
		c.Next()
	}
}

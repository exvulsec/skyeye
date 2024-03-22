package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/model"
)

const APIKEY = "apikey"

func CheckAPIKEY() gin.HandlerFunc {
	return func(c *gin.Context) {
		url := c.Request.URL.String()
		if strings.Contains(url, "/api/v1/cmc") {
			if c.Query(APIKEY) != config.Conf.HTTPServer.APIKeyForCMC {
				c.AbortWithStatusJSON(http.StatusOK, model.Message{
					Code: http.StatusUnauthorized,
					Msg:  "invalid api key",
				})
			}
		} else {
			if c.Query(APIKEY) != config.Conf.HTTPServer.APIKey {
				c.AbortWithStatusJSON(http.StatusOK, model.Message{
					Code: http.StatusUnauthorized,
					Msg:  "invalid api key",
				})
			}
		}
		c.Next()
	}
}

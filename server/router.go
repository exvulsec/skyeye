package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"openapi/controller"
)

func addRouters(r gin.IRouter) {
	addHealthRouter(r)
	apiV1 := setV1Group(r)
	addrCtrl := controller.AddressController{}
	addrCtrl.Routers(apiV1)
}

func setV1Group(r gin.IRouter) gin.IRouter {
	return r.Group("/api/v1")
}

func addHealthRouter(r gin.IRouter) {
	r.GET("/health", func(context *gin.Context) {
		context.JSON(http.StatusOK, fmt.Sprintf("running on %v", time.Now()))
	})
}

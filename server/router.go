package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/exvulsec/skyeye/http/controller"
	"github.com/exvulsec/skyeye/middleware"
)

func addRouters(r gin.IRouter) {
	addHealthRouter(r)
	apiV1 := setV1Group(r)
	monitorCTRL := controller.MonitorController{}
	monitorCTRL.Routers(apiV1)

	addressCTRL := &controller.AddressController{}
	addressCTRL.Routers(apiV1)

	ctrls := []controller.Controller{
		&controller.SkyEyeController{},
		&controller.TXController{},
		&controller.SignatureController{},
	}
	apiV1.Use(middleware.CheckAPIKEY())
	for _, ctrl := range ctrls {
		ctrl.Routers(apiV1)
	}
}

func setV1Group(r gin.IRouter) gin.IRouter {
	return r.Group("/api/v1")
}

func addHealthRouter(r gin.IRouter) {
	r.GET("/health", func(context *gin.Context) {
		context.JSON(http.StatusOK, fmt.Sprintf("running on %v", time.Now()))
	})
}

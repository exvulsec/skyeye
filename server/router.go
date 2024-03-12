package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"go-etl/http/controller"
	"go-etl/middleware"
)

func addRouters(r gin.IRouter) {
	addHealthRouter(r)
	apiV1 := setV1Group(r)
	auditController := controller.AuditController{}
	ctrls := []controller.Controller{
		&controller.AddressController{},
		&controller.MonitorController{},
		&controller.SkyEyeController{},
		&controller.TXController{},
		&controller.CMCController{AuditController: auditController},
		&auditController,
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

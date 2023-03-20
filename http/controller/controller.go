package controller

import "github.com/gin-gonic/gin"

type Controller interface {
	Routers(routers gin.IRouter)
}

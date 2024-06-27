package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
)

type HTTPServer struct {
	srv     *http.Server
	routers *gin.Engine
}

func NewHTTPServer() HTTPServer {
	r := gin.Default()
	r.Use(cors.Default())
	addRouters(r)
	s := HTTPServer{routers: r}

	s.srv = &http.Server{
		Addr: fmt.Sprintf("%s:%d",
			config.Conf.HTTPServer.Host,
			config.Conf.HTTPServer.Port),
		Handler: s.routers,
	}
	return s
}

func (s *HTTPServer) Run() {
	logrus.Info("listen addr: ", s.srv.Addr)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Errorf("listen: %v", err)
		}
	}()
	s.gracefullyShutDown()
}

func (s *HTTPServer) gracefullyShutDown() {
	// Wait for interrupt signal to gracefully shut down the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(ctx); err != nil {
		logrus.Info("server forced to shutdown:", err)
	}

	logrus.Info("server closed")
}

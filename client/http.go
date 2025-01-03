package client

import (
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
)

type HTTPInstance struct {
	initializer func() any
	instance    any
	once        sync.Once
}

var httpClient *HTTPInstance

func (ei *HTTPInstance) Instance() any {
	ei.once.Do(func() {
		ei.instance = ei.initializer()
	})
	return ei.instance
}

func initHTTPClient() any {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConnsPerHost: config.Conf.HTTPServer.ClientMaxConns,
		MaxIdleConns:        config.Conf.HTTPServer.ClientMaxConns,
		MaxConnsPerHost:     config.Conf.HTTPServer.ClientMaxConns,
	}
	logrus.Infof("init http client")
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}

func HTTPClient() *http.Client {
	return httpClient.Instance().(*http.Client)
}

func init() {
	httpClient = &HTTPInstance{initializer: initHTTPClient}
}

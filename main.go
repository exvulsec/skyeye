package main

import (
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/cmd"
)

func main() {
	defer func() {
		panicResp := recover()
		logrus.Error("got an panic err: %v", panicResp)
	}()
	cmd.Execute()
}

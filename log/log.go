package log

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

func InitLog(logPath string) {
	if logPath == "" {
		logPath = "./tmp_log.log"
	}

	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		logrus.Fatal(err)
	}
	mw := io.MultiWriter(os.Stdout, file)
	logrus.SetOutput(mw)
}

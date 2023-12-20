package utils

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"go-etl/config"
)

func ReadBlockNumberFromFile(filePath string) int64 {
	checkFileIfNotExist(filePath)
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		logrus.Fatalf("failed to read the last block number from %s", config.Conf.ETL.PreviousFile)
	}
	lastBlockNumber, err := strconv.ParseInt(strings.Trim(string(bytes), "\n"), 10, 64)
	if err != nil {
		logrus.Fatalf("failed to convert int the last block number from file %s is err: %v", filePath, err)
	}
	return lastBlockNumber
}

func WriteBlockNumberToFile(filePath string, blockNumber int64) {
	blockNumberString := strconv.FormatInt(blockNumber, 10)
	err := os.WriteFile(filePath, []byte(blockNumberString), 0777)
	if err != nil {
		logrus.Errorf("failed to write blocknumber %d to file, err is %v", blockNumber, err)
	}
}

func checkFileIfNotExist(filePath string) {
	_, err := os.Stat(filePath)
	if err == nil {
		return
	}
	if os.IsNotExist(err) {
		fi, err := os.Create(filePath)
		if err != nil {
			logrus.Fatalf("failed to create file %s, err is %v", filePath, err)
		}
		if _, err := fi.Write([]byte{48}); err != nil {
			logrus.Fatalf("failed to init file %s, err is %v", filePath, err)
		}
		logrus.Infof("create file %s is successfully", filePath)
		return
	}
	logrus.Fatalf("failed to check file %s's state, err is %v", filePath, err)
}

func ComposeTableName(schema string, tableName string) string {
	if schema != "" {
		return fmt.Sprintf("%s.%s", schema, tableName)
	}
	return tableName

}

func CheckHeaderIsGZip(header http.Header) bool {
	for k, v := range header {
		if strings.ToLower(k) == "content-encoding" && strings.Contains(strings.ToLower(v[0]), "gzip") {
			return true
		}
	}
	return false
}

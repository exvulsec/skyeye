package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
)

func GetBlockNumberFromFile(filePath string) uint64 {
	checkFileIfNotExist(filePath)
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		logrus.Fatalf("failed to read the last block number from %s", config.Conf.ETL.PreviousFile)
	}
	lastBlockNumber, err := strconv.ParseInt(strings.Trim(string(bytes), "\n"), 10, 64)
	if err != nil {
		logrus.Fatalf("failed to convert int the last block number from file %s is err: %v", filePath, err)
	}
	return uint64(lastBlockNumber)
}

func GetBlockNumberFromDB() uint64 {
	var blockNumber uint64
	tableName := ComposeTableName(config.Conf.ETL.Chain, datastore.TableTransactions)
	err := datastore.DB().Table(tableName).Select("max(blknum)").Row().Scan(&blockNumber)
	if err != nil {
		logrus.Errorf("failed to convert int the last block number from db is err: %v", err)
	}
	return blockNumber
}

func WriteBlockNumberToFile(filePath string, blockNumber int64) {
	blockNumberString := strconv.FormatInt(blockNumber, 10)
	err := os.WriteFile(filePath, []byte(blockNumberString), 0o777)
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

func IsRetriableError(err error) bool {
	return strings.Contains(err.Error(), "not found") || errors.Is(err, context.DeadlineExceeded)
}

func Retry(times int64, element any, retryFunc func(element any) (any, error)) any {
	for i := range times {
		item, err := retryFunc(element)
		if err != nil && !IsRetriableError(err) {
			logrus.Errorf("get element %v is err: %v", element, err)
			break
		}
		if item != nil {
			return item
		}
		time.Sleep(1 * time.Second)
		logrus.Infof("retry %d times to get element %v", i+1, element)
	}
	logrus.Errorf("get element %d failed, drop it", element)
	return nil
}

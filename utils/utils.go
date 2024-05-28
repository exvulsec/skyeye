package utils

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/client"
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

func Retry(retryFunc func() (any, error)) any {
	const retryInterval = 5 * time.Second
	var retryTime int
	for retryTime < config.Conf.ETL.RetryTimes || config.Conf.ETL.RetryTimes == 0 {
		item, err := retryFunc()
		if err != nil && !IsRetriableError(err) {
			logrus.Errorf("failed to retrieve element: %v", err)
			break
		}
		if !IsNil(item) {
			return item
		}
		select {
		case <-time.After(retryInterval):
			logrus.Infof("retry %d times to get element %v", retryTime+1, item)
			retryTime++
		}
	}
	logrus.Errorf("failed to retrieve element after %d retries, dropping it", retryTime)
	return nil
}

func IsNil(i any) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	return v.Kind() == reflect.Ptr && v.IsNil()
}

func RemoveLeadingZeroDigits(hex string) string {
	for index := range len(hex[2:]) {
		if hex[2+index] != '0' {
			return hex[2+index:]
		}
	}
	return ""
}

func FirstUpper(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func IsContract(address string, blockNumber int64) bool {
	code, err := client.MultiEvmClient()[config.Conf.ETL.Chain].CodeAt(context.Background(), common.HexToAddress(address), big.NewInt(blockNumber))
	if err != nil {
		logrus.Errorf("get code for address %s on block %d from rpc is err: %v", address, blockNumber, err)
		return false
	}
	return len(code) != 0
}

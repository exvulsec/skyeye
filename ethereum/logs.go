package ethereum

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/model"
	"go-etl/utils"
)

type LogFilter struct {
	chain      string
	table      string
	logChannel chan types.Log
	topics     []common.Hash
	workerSize int
}

func NewLogFilter(chain, table, topicsString string, workerSize int) *LogFilter {
	topicStrings := strings.Split(topicsString, ",")
	topics := []common.Hash{}
	for _, s := range topicStrings {
		topics = append(topics, common.HexToHash(s))
	}
	return &LogFilter{
		chain:      chain,
		table:      table,
		logChannel: make(chan types.Log, 100),
		topics:     topics,
		workerSize: workerSize,
	}
}

func (lf *LogFilter) composeFilterQuery() ethereum.FilterQuery {
	return ethereum.FilterQuery{
		Topics: [][]common.Hash{lf.topics},
	}
}

func (lf *LogFilter) handleLogs(logs []types.Log) {
	tableName := utils.ComposeTableName(lf.chain, lf.table)
	modelLogs := model.Logs{}
	wg := sync.WaitGroup{}
	rwMutex := sync.RWMutex{}
	for _, log := range logs {
		wg.Add(1)
		go func(log types.Log) {
			defer wg.Done()
			modelLog := model.Log{}
			if err := modelLog.ConvertFromEthereumLog(log); err != nil {
				logrus.Errorf("convert from log to model log is err %v", err)
			}
			rwMutex.Lock()
			modelLogs = append(modelLogs, modelLog)
			rwMutex.Unlock()
		}(log)
	}
	wg.Wait()

	if err := modelLogs.CreateBatchToDB(tableName, lf.workerSize); err != nil {
		logrus.Fatalf("insert logs to db is err %v", err)
	}

}

func (lf *LogFilter) Run() {
	var latestBlockNumber uint64 = 0
	var logs = []types.Log{}
	var startTimestamp time.Time

	sub, err := client.EvmClient().SubscribeFilterLogs(context.Background(), lf.composeFilterQuery(), lf.logChannel)
	if err != nil {
		logrus.Fatalf("subscribe logs is err %v", err)
	}

	defer close(lf.logChannel)

	for {
		select {
		case err = <-sub.Err():
			logrus.Error("subscription logs is err: %v", err)
			sub.Unsubscribe()
			break
		case l := <-lf.logChannel:
			if l.BlockNumber == latestBlockNumber {
				logs = append(logs, l)
			}
			if l.BlockNumber != latestBlockNumber {
				if len(logs) > 0 {
					lf.handleLogs(logs)
					logrus.Infof("collected block nubmer %d's %d logs cost:%.2fs", latestBlockNumber, len(logs), time.Since(startTimestamp).Seconds())
				}
				latestBlockNumber = l.BlockNumber
				logrus.Infof("start to collect block nubmer %d's log", latestBlockNumber)
				startTimestamp = time.Now()
				logs = []types.Log{l}
			}
		}
	}
}

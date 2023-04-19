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
		logChannel: make(chan types.Log, 10000),
		topics:     topics,
		workerSize: workerSize,
	}
}

func (lf *LogFilter) composeFilterQuery() ethereum.FilterQuery {
	return ethereum.FilterQuery{
		Topics: [][]common.Hash{lf.topics},
	}
}

func (lf *LogFilter) handleLogs(logs []types.Log, blockTimestamp int64) error {
	tableName := utils.ComposeTableName(lf.chain, lf.table)
	modelLogs := model.Logs{}
	wg := sync.WaitGroup{}
	rwMutex := sync.RWMutex{}
	for _, log := range logs {
		wg.Add(1)
		go func(log types.Log) {
			defer wg.Done()
			modelLog := model.Log{}
			modelLog.ConvertFromEthereumLog(log, blockTimestamp)
			rwMutex.Lock()
			defer rwMutex.Unlock()
			modelLogs = append(modelLogs, modelLog)
		}(log)
	}
	wg.Wait()

	return modelLogs.CreateBatchToDB(tableName, lf.workerSize)
}

func (lf *LogFilter) Run() {
	var currentBlockNumber uint64 = 0
	var logs = []types.Log{}
	var startTimestamp time.Time
	var currentBlockTimestamp int64 = 0

	sub, err := client.EvmClient().SubscribeFilterLogs(context.Background(), lf.composeFilterQuery(), lf.logChannel)
	if err != nil {
		logrus.Fatalf("subscribe logs is err %v", err)
	}

	for {
		select {
		case err = <-sub.Err():
			close(lf.logChannel)
			sub.Unsubscribe()
			logrus.Fatalf("subscription logs is err: %v", err)
			break
		case l := <-lf.logChannel:
			if l.BlockNumber != currentBlockNumber {
				if len(logs) > 0 {
					startTimestamp = time.Now()
					logrus.Infof("start to write block nubmer %d's %d logs to db", currentBlockNumber, len(logs))
					if err = lf.handleLogs(logs, currentBlockTimestamp); err != nil {
						logrus.Fatalf("handle logs  is err %v", err)
					}
					logrus.Infof("finish to write block nubmer %d's %d logs cost:%.2fs", currentBlockNumber, len(logs), time.Since(startTimestamp).Seconds())
				}
				currentBlockNumber = l.BlockNumber
				block, err := client.EvmClient().BlockByHash(context.Background(), l.BlockHash)
				if err != nil {
					logrus.Fatalf("get block %d info is err %v", currentBlockNumber, err)
				}
				currentBlockTimestamp = int64(block.Time())
				logrus.Infof("start to collect block nubmer %d's log", currentBlockNumber)
				logs = []types.Log{}
			}
			logs = append(logs, l)
		}
	}
}

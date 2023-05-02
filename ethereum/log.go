package ethereum

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/model"
	"go-etl/utils"
)

type logExecutor struct {
	chain      string
	tableName  string
	workerSize int
	logsCh     chan []*types.Log
	items      any
	topics     []common.Hash
}

func NewLogExecutor(chain, tableName string, workers int, topics []common.Hash) Executor {
	return &logExecutor{
		chain:      chain,
		workerSize: workers,
		topics:     topics,
		tableName:  tableName,
		logsCh:     make(chan []*types.Log, 10),
	}
}

func (le *logExecutor) Run() {
	var (
		currentBlockNumber uint64
		items              = model.Logs{}
	)
	for logs := range le.logsCh {
		if len(logs) > 0 {
			if logs[0].BlockNumber != currentBlockNumber {
				currentBlockNumber = logs[0].BlockNumber
				le.items = items
				le.Enrich()
				le.Export()
				items = model.Logs{}
			} else {
				for index := range logs {
					log := logs[index]
					if le.filterLogsByTopics(log.Topics) {
						modelLog := model.Log{}
						modelLog.ConvertFromEthereumLog(*log)
						items = append(items, modelLog)
					}
				}
			}
		}
	}
}

func (le *logExecutor) Enrich() {
	logs := le.items.(model.Logs)
	if len(logs) > 0 {
		currentBlockNumber := logs[0].BlockNumber
		block, err := client.EvmClient().BlockByNumber(context.Background(), big.NewInt(currentBlockNumber))
		if err != nil {
			logrus.Fatalf("get block %d info is err %v", currentBlockNumber, err)
		}
		currentBlockTimestamp := int64(block.Time())
		for index := range logs {
			logs[index].BlockTimestamp = currentBlockTimestamp
		}
	}
	le.items = logs
}

func (le *logExecutor) Export() {
	logs := le.items.(model.Logs)
	if len(logs) > 0 {
		startTimestamp := time.Now()
		if err := logs.CreateBatchToDB(utils.ComposeTableName(le.chain, le.tableName), le.workerSize); err != nil {
			logrus.Fatalf("insert %d logs to database is err: %v", len(logs), err)
			return
		}
		logrus.Infof("insert %d logs to database, cost %.2fs", len(logs), time.Since(startTimestamp).Seconds())
	}

}

func (le *logExecutor) filterLogsByTopics(topics []common.Hash) bool {
	for _, topic := range topics {
		for _, needTopic := range le.topics {
			if topic == needTopic {
				return true
			}
		}
	}
	return false
}

func ConvertTopicsFromString(topicString string) []common.Hash {
	topics := []common.Hash{}
	for _, topic := range strings.Split(topicString, ",") {
		topics = append(topics, common.HexToHash(topic))
	}
	return topics
}

package ethereum

import (
	"context"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
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
}

func NewLogFilter(chain, table, topicsString string) *LogFilter {
	topicStrings := strings.Split(topicsString, ",")
	topics := []common.Hash{}
	for _, s := range topicStrings {
		topics = append(topics, common.HexToHash(s))
	}
	return &LogFilter{
		chain:      chain,
		table:      table,
		logChannel: make(chan types.Log, 1),
		topics:     topics,
	}
}

func (lf *LogFilter) composeFilterQuery() ethereum.FilterQuery {
	return ethereum.FilterQuery{
		Topics: [][]common.Hash{lf.topics},
	}
}

func (lf *LogFilter) Run() {
	tableName := utils.ComposeTableName(lf.chain, lf.table)
	sub := event.Resubscribe(2*time.Second, func(ctx context.Context) (event.Subscription, error) {
		return client.EvmClient().SubscribeFilterLogs(context.Background(), lf.composeFilterQuery(), lf.logChannel)
	})
	for {
		select {
		case err := <-sub.Err():
			panic(err)
		case l := <-lf.logChannel:
			log := model.Log{}
			if err := log.ConvertFromEthereumLog(l); err != nil {
				logrus.Errorf("convert from log to model log is err %v", err)
				continue
			}
			logrus.Infof("insert %s to db at block number %d", log.Address, log.BlockNumber)
			if err := log.InsertLog(tableName); err != nil {
				logrus.Errorf("insert log to db is err %v", err)
				continue
			}
		}
	}
}

package task

import (
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type tokenTransferTask struct {
	topics       []string
	monitorAddrs *model.MonitorAddrs
}

func NewTokenTransferTask(addrs *model.MonitorAddrs) Task {
	return &tokenTransferTask{
		topics: []string{
			"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
			"0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65",
			"0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c",
		},
		monitorAddrs: addrs,
	}
}

func (ttt *tokenTransferTask) Run(data any) any {
	logs, ok := data.([]types.Log)
	if !ok || len(logs) == 0 {
		return nil
	}
	ttt.DecodeTopic(logs)
	return nil
}

func (ttt *tokenTransferTask) isExisted(topics0 string) bool {
	for _, topic := range ttt.topics {
		if strings.EqualFold(topic, topics0) {
			return true
		}
	}
	return false
}

func (ttt *tokenTransferTask) DecodeTopic(logs []types.Log) {
	startTime := time.Now()

	tokenTransferLogs := []types.Log{}
	for _, l := range logs {
		if ttt.isExisted(l.Topics[0].String()) {
			tokenTransferLogs = append(tokenTransferLogs, l)
		}
	}

	if len(tokenTransferLogs) > 0 {
		for _, l := range tokenTransferLogs {
			event, err := utils.Decode(l)
			if err != nil {
				logrus.Errorf("decode logs pos %d in transaction %s is err: %v", l.Index, l.TxHash, err)
				continue
			}
			if len(event) > 0 {
				tt := model.TokenTransfer{}
				if err := tt.DecodeFromEvent(event, l, *ttt.monitorAddrs); err != nil {
					logrus.Errorf("decode logs pos %d in transaction %s is err: %v", l.Index, l.TxHash, err)
					continue
				}
			}
		}
		logrus.Infof("block: %d, decode approval logs: %d, elapsed: %s",
			tokenTransferLogs[0].BlockNumber, len(tokenTransferLogs), utils.ElapsedTime(startTime))
	}
}

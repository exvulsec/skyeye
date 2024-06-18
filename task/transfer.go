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
	topics []string
}

func NewTokenTransferTask() Task {
	return &tokenTransferTask{topics: []string{
		"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
	}}
}

func (tt *tokenTransferTask) Run(data any) any {
	logs, ok := data.([]types.Log)
	if !ok || len(logs) == 0 {
		return nil
	}
	tt.DecodeTopic(logs)
	return nil
}

func (tt *tokenTransferTask) isExisted(topics0 string) bool {
	for _, topic := range tt.topics {
		if strings.EqualFold(topic, topics0) {
			return true
		}
	}
	return false
}

func (tt *tokenTransferTask) DecodeTopic(logs []types.Log) {
	startTime := time.Now()

	tokenTransferLogs := []types.Log{}
	for _, l := range logs {
		if tt.isExisted(l.Topics[0].String()) {
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
				if err := tt.DecodeFromEvent(event, l, model.MonitorAddrs{}); err != nil {
					logrus.Errorf("decode logs pos %d in transaction %s is err: %v", l.Index, l.TxHash, err)
					continue
				}
			}
		}
		logrus.Infof("block: %d, decode approval logs: %d, elapsed: %s",
			tokenTransferLogs[0].BlockNumber, len(tokenTransferLogs), utils.ElapsedTime(startTime))
	}
}

package task

import (
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type approvalTask struct {
	topics []string
}

func NewApprovalTask() Task {
	return &approvalTask{topics: []string{
		"0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925",
		"0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31",
		"0xda9fa7c1b00402c17d0161b249b1ab8bbec047c5a52207b9c112deffd817036b",
	}}
}

func (at *approvalTask) Run(data any) any {
	logs, ok := data.([]types.Log)
	if !ok || len(logs) == 0 {
		return nil
	}
	at.DecodeTopic(logs)
	return nil
}

func (at *approvalTask) isExisted(topics0 string) bool {
	for _, topic := range at.topics {
		if strings.EqualFold(topic, topics0) {
			return true
		}
	}
	return false
}

func (at *approvalTask) DecodeTopic(logs []types.Log) {
	startTime := time.Now()

	approvalLogs := []types.Log{}
	for _, l := range logs {
		if at.isExisted(l.Topics[0].String()) {
			approvalLogs = append(approvalLogs, l)
		}
	}

	if len(approvalLogs) > 0 {
		for _, l := range approvalLogs {
			event, err := utils.Decode(l)
			if err != nil {
				logrus.Errorf("decode logs pos %d in transaction %s is err: %v", l.Index, l.TxHash, err)
				continue
			}
			if len(event) > 0 {
				a := model.Approval{}
				a.DecodeFromEvent(event, l)
			}
		}
		logrus.Infof("block: %d, decode approval logs: %d, elapsed: %s",
			approvalLogs[0].BlockNumber, len(approvalLogs), utils.ElapsedTime(startTime))
	}
}

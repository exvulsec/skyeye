package model

import (
	"context"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"

	"go-etl/client"
	"go-etl/datastore"
)

type Log struct {
	BlockNumber      int64  `json:"blockNumber" gorm:"column:blknum"`
	BlockTimestamp   int64  `json:"blockTimestamp" gorm:"column:block_timestamp"`
	TransactionHash  string `json:"transactionHash" gorm:"column:txhash"`
	TransactionIndex int64  `json:"transactionIndex" gorm:"column:txpos"`
	LogPos           int64  `json:"logPos" gorm:"column:logpos"`
	Address          string `json:"address" gorm:"column:address"`
	Topic0           string `json:"topic0" gorm:"column:topics_0"`
	TopicCount       int    `json:"topicCount" gorm:"column:n_topics"`
	Topics           string `json:"topics" gorm:"column:topics"`
	Data             string `json:"data" gorm:"column:data"`
}

type Logs []Log

func (log *Log) ConvertFromEthereumLog(l types.Log) error {
	log.LogPos = int64(l.Index)
	log.TransactionIndex = int64(l.TxIndex)
	log.Address = strings.ToLower(l.Address.String())
	log.BlockNumber = int64(l.BlockNumber)
	b, err := client.EvmClient().BlockByHash(context.Background(), l.BlockHash)
	if err != nil {
		return err
	}
	log.TransactionHash = strings.ToLower(l.TxHash.String())
	log.BlockTimestamp = int64(b.Time())
	log.Topic0 = strings.ToLower(l.Topics[0].String())
	log.TopicCount = len(l.Topics)
	topics := "["
	for index, t := range l.Topics {
		topics += strings.ToLower(t.String())
		if index != len(l.Topics)-1 {
			topics += ","
		}
	}
	topics += "]"
	log.Topics = topics
	log.Data = hexutil.Encode(l.Data)
	return nil
}

func (log *Log) InsertLog(tableName string) error {
	return datastore.DB().Table(tableName).Create(log).Error
}

func (logs *Logs) CreateBatchToDB(tableName string, worker int) error {
	result := datastore.DB().Table(tableName).CreateInBatches(logs, worker)
	return result.Error
}

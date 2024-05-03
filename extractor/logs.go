package extractor

import (
	"context"
	"math/big"
	"strings"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/exporter"
	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/task"
	"github.com/exvulsec/skyeye/utils"
)

type logsExtractor struct {
	blocks      chan uint64
	logCh       chan logChan
	latestBlock *uint64
	workers     int
	exporters   []exporter.Exporter
	topics      []common.Hash
}

type logChan struct {
	logs   []types.Log
	blocks uint64
}

func NewLogsExtractor(workers int) Extractor {
	topics := []common.Hash{}
	for _, hash := range strings.Split(config.Conf.ETL.LogHashes, ",") {
		topics = append(topics, common.HexToHash(hash))
	}
	return &logsExtractor{
		blocks:  make(chan uint64, 10),
		logCh:   make(chan logChan, 100),
		workers: workers,
		topics:  topics,
	}
}

func (le *logsExtractor) Run() {
}

func (le *logsExtractor) getFilterQuery(blockNumber uint64) ethereum.FilterQuery {
	return ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(blockNumber)),
		ToBlock:   big.NewInt(int64(blockNumber)),
		Topics:    [][]common.Hash{le.topics},
	}
}

func (le *logsExtractor) extractLogsFromBlock(blockNumber uint64) {
	startTime := time.Now()
	logs, ok := utils.Retry(func() (any, error) {
		retryContextTimeout, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer retryCancel()
		return client.MultiEvmClient()[config.Conf.ETL.Chain].FilterLogs(retryContextTimeout, le.getFilterQuery(blockNumber))
	}).([]types.Log)

	if ok {
		le.logCh <- logChan{logs: logs, blocks: blockNumber}
		logrus.Infof("block: %d, extract logs: %d, elapsed: %s",
			blockNumber,
			len(logs),
			utils.ElapsedTime(startTime))
	}
}

func (le *logsExtractor) ExtractLog() {
	concurrency := make(chan struct{}, le.workers)
	for log := range le.logCh {
		concurrency <- struct{}{}
		go func() {
			defer func() { <-concurrency }()
			le.ExecuteTask(log)
		}()
	}
}

func (le *logsExtractor) ExtractBlocks() {
	concurrency := make(chan struct{}, le.workers)
	for blockNumber := range le.blocks {
		concurrency <- struct{}{}
		go func() {
			defer func() { <-concurrency }()
			le.extractLogsFromBlock(blockNumber)
		}()
	}
}

func (le *logsExtractor) Extract(data any) {
	header, ok := data.(*types.Header)
	if ok {
		block := header.Number.Uint64()
		if le.latestBlock == nil {
			le.latestBlock = &block
			go le.extractPreviousBlocks()
		}
		le.blocks <- block
	}
}

func (le *logsExtractor) extractPreviousBlocks() {
	var startBlock uint64
	if err := model.GetApprovalPreviousBlockNumber(config.Conf.ETL.Chain, &startBlock); err != nil {
		logrus.Fatalf("get approval previous block is err: %v", err)
	}
	endBlock := *le.latestBlock
	if startBlock == 0 {
		startBlock = endBlock - 1
	} else if endBlock-config.Conf.ETL.PreviousBlockThreshold-1 > startBlock {
		startBlock = endBlock - config.Conf.ETL.PreviousBlockThreshold - 1
	}

	startBlock += 1

	logrus.Infof("process the previous blocks from %d to %d", startBlock, endBlock)
	for blockNumber := startBlock; blockNumber < endBlock; blockNumber++ {
		le.blocks <- blockNumber
	}
	logrus.Infof("process the previous blocks is finished.")
}

func (le *logsExtractor) ProcessTasks() {
	concurrency := make(chan struct{}, le.workers)
	for logs := range le.logCh {
		concurrency <- struct{}{}
		go func() {
			defer func() { <-concurrency }()
			le.ExecuteTask(logs)
		}()

	}
}

func (le *logsExtractor) ExecuteTask(logCh logChan) {
	tasks := []task.Task{}
	var data any = logCh.logs
	for _, t := range tasks {
		go t.Do(data)
	}
}

package executor

import (
	"os"
	"sort"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
)

type fileExecutor struct {
	items               chan any
	latestBlockNumberCh chan int64
	latestBlockNumber   int64
	waitList            []int64
	filePath            string
}

func NewFileExecutor(latestBlockNumberCh chan int64) Executor {
	return &fileExecutor{
		items:               make(chan any, 10),
		filePath:            config.Conf.ETL.PreviousFile,
		latestBlockNumberCh: latestBlockNumberCh,
		waitList:            []int64{},
	}
}

func (fe *fileExecutor) Name() string {
	return "fileExecutor"
}

func (fe *fileExecutor) GetItemsCh() chan any {
	return fe.items
}

func (fe *fileExecutor) Execute() {
	select {
	case v := <-fe.latestBlockNumberCh:
		fe.latestBlockNumber = v
		for item := range fe.items {
			blockNumber := item.(int64)
			if fe.isLatestBlock(blockNumber) {
				fe.WriteBlockNumberToFile(blockNumber)
				fe.latestBlockNumber = blockNumber
			} else {
				fe.waitList = append(fe.waitList, blockNumber)
				sort.SliceStable(fe.waitList, func(i, j int) bool {
					return fe.waitList[i] < fe.waitList[j]
				})
				fe.writeBlockFromWaitList()
			}

		}
	}
}

func (fe *fileExecutor) writeBlockFromWaitList() {
	var index int
	for index < len(fe.waitList) {
		waitBlock := fe.waitList[index]
		if !fe.isLatestBlock(waitBlock) {
			break
		}
		fe.WriteBlockNumberToFile(waitBlock)
		fe.latestBlockNumber = waitBlock
		index += 1
	}
	fe.waitList = fe.waitList[index:]
}

func (fe *fileExecutor) isLatestBlock(blockNumber int64) bool {
	return fe.latestBlockNumber+1 == blockNumber
}

func (fe *fileExecutor) WriteBlockNumberToFile(blockNumber int64) {
	blockNumberString := strconv.FormatInt(blockNumber, 10)
	err := os.WriteFile(fe.filePath, []byte(blockNumberString), 0o777)
	if err != nil {
		logrus.Panicf("failed to write blocknumber %d to file, err is %v", blockNumber, err)
	}
}

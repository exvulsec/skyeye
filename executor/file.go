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
			if fe.writeBlock(blockNumber) {
				for index, waitBlock := range fe.waitList {
					if !fe.writeBlock(waitBlock) {
						fe.waitList = fe.waitList[index:]
						break
					}
				}
			} else {
				fe.waitList = append(fe.waitList, blockNumber)
				sort.SliceStable(fe.waitList, func(i, j int) bool {
					return fe.waitList[i] < fe.waitList[j]
				})
			}
		}
	}
}

func (fe *fileExecutor) writeBlock(blockNumber int64) bool {
	if fe.latestBlockNumber+1 == blockNumber {
		fe.WriteBlockNumberToFile(blockNumber)
		fe.latestBlockNumber = blockNumber
		return true
	}
	return false
}

func (fe *fileExecutor) WriteBlockNumberToFile(blockNumber int64) {
	blockNumberString := strconv.FormatInt(blockNumber, 10)
	err := os.WriteFile(fe.filePath, []byte(blockNumberString), 0o777)
	if err != nil {
		logrus.Panicf("failed to write blocknumber %d to file, err is %v", blockNumber, err)
	}
}

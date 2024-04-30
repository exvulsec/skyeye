package exporter

import (
	"os"
	"sort"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
)

type blockToFileExporter struct {
	latestBlockNumber uint64
	waitList          []uint64
	filePath          string
}

func NewBlockToFileExporter(latestBlockNumber uint64) Exporter {
	return &blockToFileExporter{
		latestBlockNumber: latestBlockNumber,
		filePath:          config.Conf.ETL.PreviousFile,
		waitList:          []uint64{},
	}
}

func (fe *blockToFileExporter) Export(data any) {
	blockNumber := data.(uint64)
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

func (fe *blockToFileExporter) writeBlockFromWaitList() {
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

func (fe *blockToFileExporter) isLatestBlock(blockNumber uint64) bool {
	return fe.latestBlockNumber+1 == blockNumber
}

func (fe *blockToFileExporter) WriteBlockNumberToFile(blockNumber uint64) {
	blockNumberString := strconv.FormatUint(blockNumber, 10)
	err := os.WriteFile(fe.filePath, []byte(blockNumberString), 0o777)
	if err != nil {
		logrus.Panicf("failed to write blocknumber %d to file, err is %v", blockNumber, err)
	}
}

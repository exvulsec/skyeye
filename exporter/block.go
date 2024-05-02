package exporter

import (
	"os"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
)

type blockToFileExporter struct {
	latestBlockNumber uint64
	filePath          string
}

func NewBlockToFileExporter(latestBlockNumber uint64) Exporter {
	return &blockToFileExporter{
		latestBlockNumber: latestBlockNumber,
		filePath:          config.Conf.ETL.PreviousFile,
	}
}

func (fe *blockToFileExporter) Export(data any) {
	blockNumber := data.(uint64)
	if fe.isLatestBlock(blockNumber) {
		fe.WriteBlockNumberToFile(blockNumber)
		fe.latestBlockNumber = blockNumber
	}
}

func (fe *blockToFileExporter) isLatestBlock(blockNumber uint64) bool {
	return fe.latestBlockNumber < blockNumber
}

func (fe *blockToFileExporter) WriteBlockNumberToFile(blockNumber uint64) {
	blockNumberString := strconv.FormatUint(blockNumber, 10)
	err := os.WriteFile(fe.filePath, []byte(blockNumberString), 0o777)
	if err != nil {
		logrus.Panicf("failed to write blocknumber %d to file, err is %v", blockNumber, err)
	}
}

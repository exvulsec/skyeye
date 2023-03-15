package ethereum

import (
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"go-etl/config"
	"go-etl/database"
	"go-etl/model"
	"go-etl/utils"
)

type contractCreationExecutor struct {
	blocks      chan types.Block
	workers     chan int64
	items       any
	txExecutor  Executor
	blockNumber int64
}

func NewContractCreationExecutor(blocks chan types.Block) Executor {
	return &contractCreationExecutor{
		blocks:     blocks,
		workers:    make(chan int64, config.Conf.ETLConfig.Worker),
		txExecutor: NewTransactionExecutor(blocks),
	}
}

func (ce *contractCreationExecutor) ExtractByBlock(block types.Block) any {
	ce.items = ce.txExecutor.ExtractByBlock(block)
	return ce.items
}

func (ce *contractCreationExecutor) Export() {
	startTimestamp := time.Now()
	txs := ce.items.(model.Transactions)
	txs.CreateBatchToDB(utils.ComposeTableName(
		config.Conf.ETLConfig.Chain,
		database.TableContractCreationTxs),
		config.Conf.Postgresql.MaxOpenConns,
	)
	logrus.Infof("insert %d txs into database cost: %.2f", len(txs), time.Since(startTimestamp).Seconds())
	utils.WriteBlockNumberToFile(config.Conf.ETLConfig.PreviousFile, ce.blockNumber)
}

func (ce *contractCreationExecutor) filterContractCreationTxs() {
	contractCreationList := model.Transactions{}
	txs := ce.items.(model.Transactions)
	for _, tx := range txs {
		if tx.ToAddress == nil {
			contractCreationList = append(contractCreationList, tx)
		}
	}
	ce.items = contractCreationList
	logrus.Infof("filter %d contract createion txs", len(contractCreationList))
}

func (ce *contractCreationExecutor) Enrich() {
	startTimestamp := time.Now()
	ce.items.(model.Transactions).EnrichReceipts(ce.workers)
	logrus.Infof("enrich %d txs from receipt cost: %.2fs",
		len(ce.items.(model.Transactions)),
		time.Since(startTimestamp).Seconds())
}

func (ce *contractCreationExecutor) Run() {
	for block := range ce.blocks {
		ce.blockNumber = block.Number().Int64()
		ce.items = ce.txExecutor.ExtractByBlock(block)
		ce.filterContractCreationTxs()
		if len(ce.items.(model.Transactions)) > 0 {
			ce.Enrich()
			ce.Export()
		}
	}
}

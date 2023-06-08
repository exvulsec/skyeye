package ethereum

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/ethereum/exporter"
	"go-etl/model"
	etlRPC "go-etl/rpc"
	"go-etl/utils"
)

type transactionExecutor struct {
	blockExecutor BlockExecutor
	logExecutor   Executor
	blockNumber   int64
	workers       int
	batchSize     int
	items         any
	isNastiff     bool
	exporters     []exporter.Exporter
}

func NewTransactionExecutor(blockExecutor BlockExecutor,
	logExecutor Executor,
	chain, openapi string,
	workers, batchSize int,
	isNastiff bool) Executor {
	return &transactionExecutor{
		blockExecutor: blockExecutor,
		logExecutor:   logExecutor,
		workers:       workers,
		batchSize:     batchSize,
		isNastiff:     isNastiff,
		exporters:     exporter.NewTransactionExporters(chain, openapi, isNastiff),
	}
}

func (te *transactionExecutor) Run() {
	go te.blockExecutor.GetBlocks()
	if te.logExecutor != nil {
		go te.logExecutor.Run()
	}

	for blockNumber := range te.blockExecutor.blocks {
		calls := []rpc.BatchElem{{
			Method: utils.RPCNameEthGetBlockByNumber,
			Args:   []any{hexutil.EncodeUint64(blockNumber), true},
			Result: &json.RawMessage{},
		}}
		client.RPCClient().MultiCall(calls, te.batchSize)
		if result, _ := calls[0].Result.(*json.RawMessage); string(*result) == "null" {
			retry := 1
			for {
				time.Sleep(1 * time.Second)
				logrus.Infof("retry %d to get block: %d info", retry, blockNumber)
				if err := client.RPCClient().Client.BatchCall(calls); err != nil {
					logrus.Errorf("batch call is err %v", err)
					return
				}
				if result, _ = calls[0].Result.(*json.RawMessage); string(*result) != "null" {
					break
				}
				retry++
			}
		}
		block, err := etlRPC.GetBlock(*calls[0].Result.(*json.RawMessage))
		if err != nil {
			logrus.Fatalf("get block from raw json message is err: %v, item is %+v", err, blockNumber)
		}
		te.blockNumber = block.Number().Int64()
		te.items = te.ExtractByBlock(*block)
		te.Enrich()
		te.Export()
	}
}

func (te *transactionExecutor) ExtractByBlock(block types.Block) any {
	startTimestamp := time.Now()
	txs := model.Transactions{}
	rwMutex := sync.RWMutex{}
	wg := sync.WaitGroup{}

	for index, tx := range block.Transactions() {
		if tx == nil {
			continue
		}
		transaction := tx
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			t := model.Transaction{}
			t.ConvertFromBlock(transaction)
			t.BlockNumber = block.Number().Int64()
			t.BlockTimestamp = int64(block.Time())
			if block.BaseFee() != nil {
				t.GasBase = decimal.NewFromBigInt(block.BaseFee(), 0)
			}
			rwMutex.Lock()
			txs = append(txs, t)
			rwMutex.Unlock()
		}(index)
	}
	wg.Wait()
	logrus.Infof("extract %d txs from block %d cost %.2fs",
		len(block.Transactions()),
		block.Number(),
		time.Since(startTimestamp).Seconds())
	return txs
}

func (te *transactionExecutor) Enrich() {
	startTimestamp := time.Now()
	te.filterTransactions()
	txs := te.items.(model.Transactions)
	if len(txs) > 0 {
		var logCh chan []*types.Log
		if te.logExecutor != nil {
			lge, _ := te.logExecutor.(*logExecutor)
			logCh = lge.logsCh
		}

		txs.EnrichReceipts(te.batchSize, te.workers, logCh)
		logrus.Infof("enrich %d txs from receipt cost: %.2fs", len(txs), time.Since(startTimestamp).Seconds())
	}
}

func (te *transactionExecutor) Export() {
	txs := te.items.(model.Transactions)
	if len(txs) > 0 {
		for _, e := range te.exporters {
			e.ExportItems(txs)
		}
	}
	utils.WriteBlockNumberToFile(config.Conf.ETL.PreviousFile, te.blockNumber)
}

func (te *transactionExecutor) filterTransactions() {
	if te.isNastiff {
		txs := model.Transactions{}
		for _, item := range te.items.(model.Transactions) {
			if item.ToAddress == nil {
				txs = append(txs, item)
			}
		}
		te.items = txs
	}
}

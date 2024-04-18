package model

import (
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/shopspring/decimal"

	"github.com/exvulsec/skyeye/config"
)

const (
	EVMPlatformCurrency = "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
)

type TransactionTraceBase struct {
	From     string `json:"from"`
	Gas      string `json:"gas"`
	GasUsed  string `json:"gasUsed"`
	To       string `json:"to"`
	Input    string `json:"input"`
	Output   string `json:"output"`
	Value    string `json:"value"`
	CallType string `json:"type"`
}

type TransactionTrace struct {
	TransactionTraceBase
	Calls []TransactionTrace `json:"calls"`
	Depth []int
}

type Queue []TransactionTrace

func (q *Queue) Push(trace TransactionTrace) {
	*q = append(*q, trace)
}

func (q *Queue) Pop() *TransactionTrace {
	if !q.IsEmpty() {
		top := (*q)[0]
		*q = (*q)[1:]
		return &top
	}
	return nil
}

func (q *Queue) IsEmpty() bool {
	return len(*q) == 0
}

func (trace *TransactionTrace) GetContractAddress() (string, bool) {
	if trace.CallType == "CREATE" || trace.CallType == "CREATE2" {
		if trace.FilterAddress(trace.TransactionTraceBase.From) {
			return "", true
		}
		return trace.TransactionTraceBase.To, false
	}
	return "", false
}

func (trace *TransactionTrace) ListContracts() ([]string, bool) {
	queue := Queue{}
	queue.Push(*trace)
	contracts := []string{}
	for {
		if queue.IsEmpty() {
			break
		}
		txTrace := queue.Pop()
		address, skip := txTrace.GetContractAddress()
		if skip {
			return contracts, true
		}
		if address != "" {
			contracts = append(contracts, address)
		}
		for index := range txTrace.Calls {
			call := txTrace.Calls[index]
			call.Depth = append(txTrace.Depth, index)
			queue.Push(call)
		}
	}
	return contracts, false
}

func (trace *TransactionTrace) ListTransferEvent() []AssetTransfer {
	assetTransfers := []AssetTransfer{}
	queue := Queue{}
	queue.Push(*trace)
	for {
		if queue.IsEmpty() {
			break
		}
		trace := queue.Pop()
		if trace != nil {
			if !strings.EqualFold(trace.Value, "0x0") && trace.Value != "" {
				value, err := hexutil.DecodeBig(trace.Value)
				if err != nil {
					panic(err)
				}
				newValue := decimal.NewFromBigInt(value, 0)
				assetTransfers = append(assetTransfers, AssetTransfer{
					From:    trace.From,
					To:      trace.To,
					Address: EVMPlatformCurrency,
					Value:   newValue,
				})
			}
			for _, call := range trace.Calls {
				queue.Push(call)
			}
		}
	}
	return assetTransfers
}

func (trace *TransactionTrace) FilterAddress(addr string) bool {
	contracts := strings.Split(config.Conf.ETL.FilterContracts, ",")
	for _, contract := range contracts {
		if strings.EqualFold(contract, addr) {
			return true
		}
	}
	return false
}

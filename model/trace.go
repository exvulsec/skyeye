package model

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/shopspring/decimal"

	"github.com/exvulsec/skyeye/config"
)

const (
	EVMPlatformCurrency = "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
)

type TransactionTraceLog struct {
	Address common.Address `json:"address"`
	Topics  []common.Hash  `json:"topics"`
	Data    string         `json:"data"`
	Index   uint           `json:"index"`
}

type TransactionTraceCall struct {
	From     string                 `json:"from"`
	Gas      string                 `json:"gas"`
	GasUsed  string                 `json:"gasUsed"`
	To       string                 `json:"to"`
	Input    string                 `json:"input"`
	Output   string                 `json:"output"`
	Value    string                 `json:"value"`
	CallType string                 `json:"type"`
	Error    string                 `json:"error"`
	Calls    []TransactionTraceCall `json:"calls"`
	Logs     []TransactionTraceLog  `json:"logs"`
}

func (call *TransactionTraceCall) GetContractAddress() (string, bool) {
	if call.Error != "" {
		return "", true
	}
	if call.CallType == "CREATE" || call.CallType == "CREATE2" {
		if call.FilterAddress(call.From) {
			return "", true
		}
		return call.To, false
	}
	return "", false
}

func (call *TransactionTraceCall) ListContracts() ([]string, bool) {
	queue := Queue[*TransactionTraceCall]{}
	queue.Push(call)
	contracts := []string{}
	for {
		if queue.IsEmpty() {
			break
		}
		txTrace := queue.Pop()
		if txTrace == nil {
			continue
		}
		address, skip := txTrace.GetContractAddress()
		if skip {
			return contracts, true
		}
		if address != "" {
			contracts = append(contracts, address)
		}
		for index := range txTrace.Calls {
			call := txTrace.Calls[index]
			queue.Push(&call)
		}
	}
	return contracts, false
}

func (call *TransactionTraceCall) ListTransferEvent() []AssetTransfer {
	assetTransfers := []AssetTransfer{}
	queue := Queue[*TransactionTraceCall]{}
	queue.Push(call)
	for {
		if queue.IsEmpty() {
			break
		}
		call := queue.Pop()
		if call != nil {
			if call.Error != "" {
				continue
			}
			if !strings.EqualFold(call.Value, "0x0") && call.Value != "" {
				value, err := hexutil.DecodeBig(call.Value)
				if err != nil {
					panic(err)
				}
				newValue := decimal.NewFromBigInt(value, 0)
				assetTransfers = append(assetTransfers, AssetTransfer{
					From:    call.From,
					To:      call.To,
					Address: EVMPlatformCurrency,
					Value:   newValue,
				})
			}
			for _, call := range call.Calls {
				queue.Push(&call)
			}
		}
	}
	return assetTransfers
}

func (call *TransactionTraceCall) ListTransferEventWithDFS(assetTransfers AssetTransfers, txhash string) AssetTransfers {
	if call.Error != "" {
		return assetTransfers
	}
	if !strings.EqualFold(call.Value, "0x0") && call.Value != "" {
		value, err := hexutil.DecodeBig(call.Value)
		if err != nil {
			panic(err)
		}
		newValue := decimal.NewFromBigInt(value, 0)
		assetTransfers = append(assetTransfers, AssetTransfer{
			From:    call.From,
			To:      call.To,
			Address: EVMPlatformCurrency,
			Value:   newValue,
		})
	}
	for _, c := range call.Calls {
		assetTransfers = c.ListTransferEventWithDFS(assetTransfers, txhash)
	}
	assetTransfers.ComposeFromTraceLogs(call.Logs, txhash)
	return assetTransfers
}

func (call *TransactionTraceCall) FilterAddress(addr string) bool {
	contracts := strings.Split(config.Conf.ETL.FilterContracts, ",")
	for _, contract := range contracts {
		if strings.EqualFold(contract, addr) {
			return true
		}
	}
	return false
}

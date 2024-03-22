package model

import (
	"strings"

	"github.com/exvulsec/skyeye/config"
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

func (trace *TransactionTrace) GetContractAddress() string {
	if trace.CallType == "CREATE" || trace.CallType == "CREATE2" {
		if trace.FilterAddress(trace.TransactionTraceBase.From) {
			return ""
		}
		return trace.TransactionTraceBase.To
	}
	return ""
}

func (trace *TransactionTrace) ListContracts() []string {
	queue := Queue{}
	queue.Push(*trace)
	contracts := []string{}
	for {
		if queue.IsEmpty() {
			break
		}
		txTrace := queue.Pop()
		address := txTrace.GetContractAddress()
		if address != "" {
			contracts = append(contracts, address)
		}
		for index := range txTrace.Calls {
			call := txTrace.Calls[index]
			call.Depth = append(txTrace.Depth, index)
			queue.Push(call)
		}

	}
	return contracts
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

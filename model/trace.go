package model

import (
	"context"
	"go-etl/client"
	"go-etl/utils"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/sirupsen/logrus"
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

type TransactionTraceResponse struct {
	TransactionTraceBase
	Calls []TransactionTraceResponse `json:"calls"`
	Depth []int
}

type Queue []TransactionTraceResponse

func (q *Queue) Push(trace TransactionTraceResponse) {
	*q = append(*q, trace)
}

func (q *Queue) Pop() *TransactionTraceResponse {
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

type TransactionTrace struct {
	TransactionTraceBase
	ContractAddress string `json:"contract_address"`
	TraceAddress    []int  `json:"trace_address"`
}

func (txTraceResp *TransactionTraceResponse) convertToTransactionTrace() TransactionTrace {
	txTrace := TransactionTrace{}
	txTrace.TransactionTraceBase = txTraceResp.TransactionTraceBase
	if txTraceResp.CallType == "CREATE" || txTraceResp.CallType == "CREATE2" {
		txTrace.ContractAddress = txTrace.To
		txTrace.To = ""
	}
	txTrace.TraceAddress = txTraceResp.Depth
	return txTrace
}

func (txTraceResp *TransactionTraceResponse) flatTraceTransaction() []TransactionTrace {
	queue := Queue{}
	txTraceResp.Depth = []int{}
	queue.Push(*txTraceResp)
	traces := []TransactionTrace{}
	for {
		if queue.IsEmpty() {
			break
		}
		txTrace := queue.Pop()
		if txTrace != nil {
			traces = append(traces, txTrace.convertToTransactionTrace())
		}
		for index := range txTrace.Calls {
			call := txTrace.Calls[index]
			call.Depth = append(txTrace.Depth, index)
			queue.Push(call)
		}

	}

	return traces
}

func GetTransactionTrace(txHash string) []TransactionTrace {
	ctxWithTimeOut, cancel := context.WithTimeout(context.TODO(), 20*time.Second)
	defer cancel()
	var txTraceResp *TransactionTraceResponse
	for i := 0; i < 5; i++ {
		err := client.EvmClient().Client().CallContext(ctxWithTimeOut, &txTraceResp,
			"debug_traceTransaction",
			common.HexToHash(txHash),
			map[string]string{
				"tracer": "callTracer",
			})
		if err != nil && !utils.IsRetriableError(err) {
			logrus.Errorf("get tx trace for tx %s is err %v", txHash, err)
			return []TransactionTrace{}
		}
		if txTraceResp != nil {
			break
		}
		time.Sleep(1 * time.Second)
		logrus.Infof("retry %d times to get tx's trace %s", i+1, txHash)
	}
	if txTraceResp == nil {
		logrus.Infof("get receipt with txhash %s failed, drop it", txHash)
		return []TransactionTrace{}
	}
	return txTraceResp.flatTraceTransaction()
}

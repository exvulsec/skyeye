package executor

import (
	"github.com/exvulsec/skyeye/model"
)

type transactionExecutor struct {
	items chan any
}

func NewTransactionExecutor() Executor {
	return &transactionExecutor{
		items: make(chan any, 10),
	}
}

func (te *transactionExecutor) GetItemsCh() chan any {
	return te.items
}

func (te *transactionExecutor) Execute() {
	for item := range te.items {
		txs, ok := item.(model.Transactions)
		if ok {
			txs.EvaluateContractCreation()
		}
	}
}

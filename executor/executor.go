package executor

import (
	"sync"
)

type Executor interface {
	Execute(data any)
}

type MultiProcessExecutor struct {
	Data        interface{}
	NumWorkers  int
	ProcessFunc ProcessFunc
}

type ProcessFunc func(any)

func (e *MultiProcessExecutor) Run() {
	var wg sync.WaitGroup

	for i := 0; i < e.NumWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.ProcessFunc(e.Data)
		}()
	}
	wg.Wait()
}

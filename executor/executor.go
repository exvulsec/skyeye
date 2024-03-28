package executor

type Executor interface {
	Execute(workerID int)
	GetItemsCh() chan any
}

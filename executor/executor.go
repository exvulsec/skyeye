package executor

type Executor interface {
	Name() string
	Execute(workerID int)
	GetItemsCh() chan any
}

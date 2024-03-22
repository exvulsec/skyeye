package executor

type Executor interface {
	Execute()
	GetItemsCh() chan any
}

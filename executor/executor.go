package executor

type Executor interface {
	Name() string
	Execute()
	GetItemsCh() chan any
}

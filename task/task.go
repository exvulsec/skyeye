package task

type Task interface {
	Do(data any) any
	Done() bool
}

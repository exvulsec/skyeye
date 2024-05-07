package task

type Task interface {
	Run(data any) any
}

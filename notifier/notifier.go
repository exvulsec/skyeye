package notifier

const LarkNotifierName = "LarkNotifier"

type Notifier interface {
	Name() string
	Notify(data any)
}

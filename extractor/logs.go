package extractor

//type logExecutor struct {
//	items     chan any
//	workers   int
//	executors []Executor
//}
//
//func NewLogExecutor(workers int) Executor {
//	return &logExecutor{
//		items:     make(chan any, 10),
//		workers:   workers,
//		executors: []Executor{NewAssetExecutor(workers, latestBlockNumberCh)},
//	}
//}
//
//func (le *logExecutor) Name() string {
//	return "Log"
//}
//
//func (le *logExecutor) GetItemsCh() chan any {
//	return ce.items
//}
//
//func (le *logExecutor) Execute() {
//	for _, e := range le.executors {
//		go e.Execute()
//	}
//	for range le.workers {
//		go func() {
//			for item := range le.items {
//			}
//		}()
//	}
//}

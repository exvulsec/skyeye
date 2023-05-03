package ethereum

type Executor interface {
	Run()
	Export()
	Enrich()
}

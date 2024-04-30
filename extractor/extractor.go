package extractor

type Extractor interface {
	Run()
	Extract(data any)
}

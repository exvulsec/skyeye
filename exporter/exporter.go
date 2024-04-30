package exporter

type Exporter interface {
	Export(data any)
}

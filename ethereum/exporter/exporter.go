package exporter

type Exporter interface {
	ExportItems(items any)
}

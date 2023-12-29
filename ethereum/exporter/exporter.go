package exporter

import "go-etl/config"

type Exporter interface {
	ExportItems(items any)
}

func NewTransactionExporters(chain, openAPIServer string, isNastiff bool, batchSize int) []Exporter {
	exporters := []Exporter{NewTransactionPostgresqlExporter(chain)}
	if isNastiff {
		exporters = append(exporters, NewNastiffTransferExporter(chain, openAPIServer, config.Conf.ETL.ScanInterval, batchSize))
	}
	return exporters
}

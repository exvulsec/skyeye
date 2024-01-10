package exporter

import (
	"go-etl/config"
)

type Exporter interface {
	Run()
	GetItemsCh() chan any
}

func NewTransactionExporters(chain, openAPIServer string, isNastiff bool, workers int) []Exporter {
	exporters := []Exporter{NewTransactionPostgresqlExporter(chain, workers)}
	if isNastiff {
		exporters = append(exporters, NewSkyEyeExporter(chain, openAPIServer, config.Conf.ETL.ScanInterval, workers))
	}
	return exporters
}

func StartExporters(exporters []Exporter) {
	for _, e := range exporters {
		go e.Run()
	}
}

func WriteDataToExporters(exporters []Exporter, data any) {
	for _, e := range exporters {
		ch := e.GetItemsCh()
		ch <- data
	}
}

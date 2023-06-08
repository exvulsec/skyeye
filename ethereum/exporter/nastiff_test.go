package exporter

import (
	"strings"
	"testing"

	"go-etl/config"
	"go-etl/model"
)

func TestNastiffTransactionExporter_SendToTelegram(t *testing.T) {
	config.SetupConfig("../../config/config.bsc.yaml")
	e := NewNastiffTransferExporter("bsc", "http://47.243.70.228:8088", 0)
	addrLabelString := "Binance_0xe9e7,JetswapFactory,JetswapRouter,PancakeSwap: Router v2,StrategyWingsLP,StrategyWingsLP, 0x{2}"
	addrLabels := strings.Split(addrLabelString, ",")
	tx := model.NastiffTransaction{
		Chain:           "bsc",
		BlockNumber:     28915853,
		BlockTimestamp:  1686209690,
		Score:           77,
		Push20Args:      addrLabels,
		TxHash:          "0xfe0d401ca3df44d11b5384cc19cf7d04864ce795fd49915a7fc7e58f12b171f4",
		FromAddress:     "0xa8e889055b80483a3572c9860e5b50e351cafb51",
		ContractAddress: "0x7dc00e2d8bef2d3e5e93170a76aea77043e63368",
	}
	nte := e.(*NastiffTransactionExporter)
	if err := nte.SendToTelegram(tx); err != nil {
		panic(err)
	}
}

package utils

import "strings"

const (
	EtherScanAPI               = "https://api.etherscan.io/api?module=account&apikey=%s&address=%s&startblock=0&endblock=99999999&sort=asc&action=%s&page=1&offset=1"
	EtherScanTransactionAction = "txlist"
	EtherScanTraceAction       = "txlistinternal"
	EtherScanGenesisAddress    = "GENESIS"
)

func GetScanAPI(chain string) string {
	switch strings.ToLower(chain) {
	case ChainEthereum:
		return EtherScanAPI
	default:
		return EtherScanAPI
	}
}

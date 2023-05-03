package utils

import "strings"

const (
	EtherScanAPIURL       = "https://api.etherscan.io/api"
	BSCScanAPIURL         = "https://api.bscscan.com/api"
	EtherScanURL          = "https://etherscan.io"
	BSCScanURL            = "https://bscscan.com"
	APIQuery              = "?module=account&apikey=%s&address=%s&startblock=0&endblock=99999999&sort=asc&action=%s&page=1&offset=1"
	ScanTransactionAction = "txlist"
	ScanTraceAction       = "txlistinternal"
	ScanGenesisAddress    = "GENESIS"
)

func GetScanAPI(chain string) string {
	switch strings.ToLower(chain) {
	case ChainEthereum:
		return EtherScanAPIURL
	case ChainBSC:
		return BSCScanAPIURL
	default:
		return EtherScanAPIURL
	}
}

func GetScanURL(chain string) string {
	switch strings.ToLower(chain) {
	case ChainEthereum:
		return EtherScanURL
	case ChainBSC:
		return BSCScanURL
	default:
		return EtherScanURL
	}
}

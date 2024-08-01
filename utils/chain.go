package utils

import "strings"

const (
	ChainEthereum  = "ethereum"
	ChainEth       = "eth"
	ChainBSC       = "bsc"
	ChainEmpty     = ""
	ChainKey       = "chain"
	ChainOptimism  = "optimism"
	ChainFantom    = "fantom"
	ChainArbitrum  = "arbitrum"
	ChainArb       = "arb"
	ChainAvalanche = "avalanche"
	ChainPolygon   = "polygon"
	ChainCelo      = "celo"
)

func GetSupportChain(chain string) string {
	switch chain {
	case ChainEth, ChainEthereum:
		return ChainEthereum
	case ChainBSC:
		return ChainBSC
	case ChainArbitrum, ChainArb:
		return ChainArbitrum
	default:
		return chain
	}
}

func ConvertChainToBlockSecChainID(chain string) string {
	switch chain {
	case ChainEthereum, ChainEth:
		return ChainEth
	case ChainBSC:
		return ChainBSC
	case ChainArbitrum, ChainArb:
		return ChainArbitrum

	default:
		return ChainEth
	}
}

func ConvertChainToDeFiHackLabChain(chain string) string {
	switch chain {
	case ChainEthereum, ChainEth:
		return ChainEth
	case ChainBSC:
		return ChainBSC
	case ChainArbitrum, ChainArb:
		return ChainArbitrum
	default:
		return ChainEth
	}
}

const (
	CGCEthereum  = ChainEthereum
	CGCBSC       = "binance-smart-chain"
	CGCArbitrum  = "arbitrum-one"
	CGCAvalanche = "avalanche"
)

func ConvertChainToCGCID(chain string) string {
	switch chain {
	case ChainBSC:
		return CGCBSC
	case ChainArbitrum:
		return CGCArbitrum
	case ChainAvalanche:
		return CGCAvalanche
	default:
		return CGCEthereum
	}
}

const (
	BSCChainCurrency      = "BNB"
	EthereumChainCurrency = "Eth"
)

func GetChainCurrency(chain string) string {
	switch strings.ToLower(chain) {
	case ChainBSC:
		return BSCChainCurrency
	case ChainEthereum:
		return EthereumChainCurrency
	}
	return ""
}

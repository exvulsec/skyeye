package utils

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

func GetChainFromQuery(chain string) string {
	switch chain {
	case ChainEmpty:
		return ChainEthereum
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

func ConvertChainToMetaDock(chain string) string {
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

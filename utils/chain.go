package utils

func GetChainFromQuery(chain string) string {
	switch chain {
	case ChainEmpty:
		return ChainEthereum
	default:
		return chain
	}
}

func ConvertChainToMetaDock(chain string) string {
	switch chain {
	case ChainEthereum:
		return "eth"
	case ChainBSC:
		return "bsc"
	default:
		return "eth"
	}
}

package utils

func GetChainFromQuery(chain string) string {
	if chain == ChainEmpty {
		return ChainEthereum
	}
	return chain
}

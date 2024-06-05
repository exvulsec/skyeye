package utils

import (
	"fmt"
	"strings"
)

type signaturesConfig struct {
	signatures []string
	threshold  int
}

var Erc20Signatures = []string{
	"18160ddd", // totalSupply()
	"70a08231", // balanceOf(address)
	"dd62ed3e", // allowance(address,address)
	"a9059cbb", // transfer(address,uint256)
	"23b872dd", // transferFrom(address,address,uint256)
	"095ea7b3", // approve(address,uint256)
}

var Erc721Signatures = []string{
	"6352211e", // ownerOf(uint256)
	"70a08231", // balanceOf(address)
	"b88d4fde", // safeTransferFrom(address,address,uint256,bytes)
	"42842e0e", // safeTransferFrom(address,address,uint256)
	"23b872dd", // transferFrom(address,address,uint256)
	"095ea7b3", // approve(address,uint256)
	"a22cb465", // setApprovalForAll(address,bool)
	"081812fc", // getApproved(uint256)
	"e985e9c5", // isApprovedForAll(address,address)
}

var Erc1155Signatures = []string{
	"00fdd58e", // balanceOf(address,uint256)
	"4e1273f4", // balanceOfBatch(address[],uint256[])
	"a22cb465", // setApprovalForAll(address,bool)
	"e985e9c5", // isApprovedForAll(address,address)
	"f242432a", // safeTransferFrom(address,address,uint256,uint256,bytes)
	"2eb2c2d6", // safeBatchTransferFrom(address,address,uint256[],uint256[],bytes)
}

var StartStopWithdrawalSignatures = []string{
	"1b55ba3a", // Start()
	"70e44c6a", // Withdrawal()
	"bedf0f4a", // Stop()
}

var KeyStopWithdrawSignatures = []string{
	"2b42b941", // SetTradeBalancePERCENT(uint256)
	"57ea89b6", // Withdraw()
	"9763d29b", // SetTradeBalanceETH(uint256)
	"bedf0f4a", // Stop()
	"eaf67ab9", // StartNative()
	"f39d8c65", // Key()
}

var StartStopWithdrawSymbolOwnerSignatures = []string{
	"1b55ba3a", // Start()
	"70e44c6a", // Withdrawal()
	"8da5cb5b", // owner()
	"95d89b41", // symbol()
	"bedf0f4a", // Stop()
}

var RugPullSignatures = []string{
	"c9567bf9", // openTrading
	"9e78fb4f", // createPair
	"751039fc", // removeLimits
	"715018a6", // renounceOwnership
	"4c8afff4", // delBots
}

var RugPullThreshold = 5

var signaturesConfigs = []signaturesConfig{
	{signatures: Erc20Signatures, threshold: 6},
	{signatures: Erc721Signatures, threshold: 9},
	{signatures: Erc1155Signatures, threshold: 6},
	{signatures: StartStopWithdrawalSignatures, threshold: 3},
	{signatures: KeyStopWithdrawSignatures, threshold: 6},
	{signatures: StartStopWithdrawSymbolOwnerSignatures, threshold: 5},
}

func IsRugPullContractType(funcSignatures []string) bool {
	count := 0
	for _, funcSign := range funcSignatures {
		for _, signature := range RugPullSignatures {
			if strings.EqualFold(fmt.Sprintf("0x%s", signature), funcSign) {
				count++
				break
			}
		}
	}
	return count == RugPullThreshold && len(funcSignatures) >= RugPullThreshold
}

func isFilterContractType(funcSignatures []string, signaturesConfig signaturesConfig) bool {
	count := 0

	for _, funcSign := range funcSignatures {
		for _, signature := range signaturesConfig.signatures {
			if strings.EqualFold(fmt.Sprintf("0x%s", signature), funcSign) {
				count++
				break
			}
		}
	}
	return count == signaturesConfig.threshold && len(funcSignatures) == signaturesConfig.threshold
}

func IsSkipContract(funcSignatures []string) bool {
	for _, signaturesConfig := range signaturesConfigs {
		if isFilterContractType(funcSignatures, signaturesConfig) {
			return true
		}
	}
	return false
}

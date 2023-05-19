package utils

import "fmt"

var Erc20Signatures = []string{
	"06fdde03", // name()
	"95d89b41", // symbol()
	"313ce567", // decimals()
	"18160ddd", // totalSupply()
	"70a08231", // balanceOf(address)
	"dd62ed3e", // allowance(address,address)
	"a9059cbb", // transfer(address,uint256)
	"23b872dd", // transferFrom(address,address,uint256)
	"095ea7b3", // approve(address,uint256)
}

var Erc721Signatures = []string{
	"06fdde03", // name()
	"95d89b41", // symbol()
	"18160ddd", // totalSupply()
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

const (
	Erc20SignatureThreshold  = 5
	Erc721SignatureThreshold = 8
)

func IsErc20Or721(signatures []string, funcSignatures []string, threshold int) bool {
	count := 0
	for _, funcSign := range funcSignatures {
		for _, sign := range signatures {
			if fmt.Sprintf("0x%s", sign) == funcSign {
				count++
				break
			}
		}
	}
	return count >= threshold
}

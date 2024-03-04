package utils

import "fmt"

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

const (
	Erc20SignatureThreshold   = 6
	Erc721SignatureThreshold  = 9
	Erc1155SignatureThreshold = 6
)

func IsToken(signatures []string, funcSignatures []string, threshold int) bool {
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

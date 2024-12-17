package model

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
)

func TestHeimdallPolicyCalc(t *testing.T) {
	config.SetupConfig("../../config/config.dev.yaml")
	data, err := hexutil.Decode("0x60806040526004361061002d5760003560e01c8063be9a655514610122578063d4e932921461012c57610034565b3661003457005b600064f86d21ed7b9050732aeef5e65385c72e985e0361baa234ff00ce199673ffffffffffffffffffffffffffffffffffffffff163273ffffffffffffffffffffffffffffffffffffffff16141561011f576198746100d76000368080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f82011690508083019250505050505050610136565b0173ffffffffffffffffffffffffffffffffffffffff166108fc479081150290604051600060405180830381858888f1935050505015801561011d573d6000803e3d6000fd5b505b50005b61012a610144565b005b610134610395565b005b600060148201519050919050565b6000645f6752a55f9050600460009054906101000a900460ff1615610183576000600460006101000a81548160ff021916908315150217905550610392565b600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166352efd685476040518263ffffffff1660e01b815260040180828152602001915050600060405180830381600087803b1580156101f857600080fd5b505af115801561020c573d6000803e3d6000fd5b505050506040513d6000823e3d601f19601f82011682018060405250602081101561023657600080fd5b810190808051604051939291908464010000000082111561025657600080fd5b8382019150602082018581111561026c57600080fd5b825186600182028301116401000000008211171561028957600080fd5b8083526020830192505050908051906020019080838360005b838110156102bd5780820151818401526020810190506102a2565b50505050905090810190601f1680156102ea5780820380516001836020036101000a031916815260200191505b506040525050506040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825283818151815260200191508051906020019080838360005b8381101561035757808201518184015260208101905061033c565b50505050905090810190601f1680156103845780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b50565b600064470abbf5259050670214e8348c4f000047101561041d57600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166108fc479081150290604051600060405180830381858888f19350505050158015610417573d6000803e3d6000fd5b5061062c565b600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1663ccc98790476040518263ffffffff1660e01b815260040180828152602001915050600060405180830381600087803b15801561049257600080fd5b505af11580156104a6573d6000803e3d6000fd5b505050506040513d6000823e3d601f19601f8201168201806040525060208110156104d057600080fd5b81019080805160405193929190846401000000008211156104f057600080fd5b8382019150602082018581111561050657600080fd5b825186600182028301116401000000008211171561052357600080fd5b8083526020830192505050908051906020019080838360005b8381101561055757808201518184015260208101905061053c565b50505050905090810190601f1680156105845780820380516001836020036101000a031916815260200191505b506040525050506040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825283818151815260200191508051906020019080838360005b838110156105f15780820151818401526020810190506105d6565b50505050905090810190601f16801561061e5780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b5056fea2646970667358221220d2e0a66ca4b633fdae5317d40489376f74dbe66287c57bcc1a6ba9c41c01aaa364736f6c63430006060033")
	if err != nil {
		logrus.Fatal(err)
	}
	tx := SkyEyeTransaction{
		ContractAddress: "0x58e3b3ac35351d3f3a51e7d63216a279662377e0",
		ByteCode:        data,
	}
	hpc := HeimdallPolicyCalc{}
	hpc.Filter(&tx)
	fmt.Println(hpc.Heimdall)
}

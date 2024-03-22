package exporter

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/model"
)

func TestNastiffTransactionExporter_SendToTelegram(t *testing.T) {
	config.SetupConfig("../../config/config.dev.yaml")
	e := NewSkyEyeExporter("ethereum", "http://localhost:8088", 0, 5)
	addrLabelString := "Binance_0xe9e7,JetswapFactory,JetswapRouter,PancakeSwap: Router v2,StrategyWingsLP,StrategyWingsLP, 0x{2}"
	addrLabels := strings.Split(addrLabelString, ",")
	funcString := "accumulatedRewardPerShare,addMinterShare,claimReward,collection,owner,pendingReward,renounceOwnership"
	funcs := strings.Split(funcString, ",")
	data, err := hexutil.Decode("0x608060405234801561001057600080fd5b50600436106100a95760003560e01c8063d5b9979711610071578063d5b99797146100e3578063eb2021c3146100e3578063ef8e5aee14610136578063f123486d14610149578063f3684ea71461015c578063fc0c546a1461016f57600080fd5b80630a9ae69d146100ae5780631fa70531146100cc5780637ed1f1dd146100e3578063c54e44eb146100f8578063d2ec010e14610123575b600080fd5b6100b6610182565b6040516100c3919061103c565b60405180910390f35b6100d560055481565b6040519081526020016100c3565b6100f66100f136600461106e565b610210565b005b60015461010b906001600160a01b031681565b6040516001600160a01b0390911681526020016100c3565b6100f661013136600461111a565b610224565b6100f66101443660046111cb565b600555565b6100f66101573660046111cb565b610234565b60005461010b906001600160a01b031681565b60025461010b906001600160a01b031681565b6004805461018f906111e4565b80601f01602080910402602001604051908101604052809291908181526020018280546101bb906111e4565b80156102085780601f106101dd57610100808354040283529160200191610208565b820191906000526020600020905b8154815290600101906020018083116101eb57829003601f168201915b505050505081565b61021d85858585856103e4565b5050505050565b6004610230828261126f565b5050565b6040805173d5f05644ef5d0a36ca8c8b5177ffbd09ec63f92f602082018190527355d398326f99059ff775485246999027b319795592820183905260608201849052919060009060800160405160208183030381529060405290506000836001600160a01b0316634a248d2a6040518163ffffffff1660e01b8152600401602060405180830381865afa1580156102cf573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906102f3919061132f565b9050826001600160a01b0316816001600160a01b03160361037857604051633429253960e21b81526001600160a01b0385169063d0a494e4906103419088906000903090889060040161134c565b600060405180830381600087803b15801561035b57600080fd5b505af115801561036f573d6000803e3d6000fd5b5050505061021d565b604051633429253960e21b81526001600160a01b0385169063d0a494e4906103ab9060009089903090889060040161134c565b600060405180830381600087803b1580156103c557600080fd5b505af11580156103d9573d6000803e3d6000fd5b505050505050505050565b600080806103f484860186611383565b919450925090506001600160a01b0388163014801561041b5750336001600160a01b038416145b6104625760405162461bcd60e51b815260206004820152601360248201527212105391131157d1931054d217d39153925151606a1b60448201526064015b60405180910390fd5b6001546000546001600160a01b039182169163095ea7b3911661048684600a6113da565b6040516001600160e01b031960e085901b1681526001600160a01b03909216600483015260248201526044016020604051808303816000875af11580156104d1573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906104f591906113f7565b506001546003546001600160a01b039182169163095ea7b3911661051a84600a6113da565b6040516001600160e01b031960e085901b1681526001600160a01b03909216600483015260248201526044016020604051808303816000875af1158015610565573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061058991906113f7565b5060025460035460405163095ea7b360e01b81526001600160a01b039182166004820152600019602482015291169063095ea7b3906044016020604051808303816000875af11580156105e0573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061060491906113f7565b50600054604051633e7098eb60e21b81526001600160a01b039091169063f9c263ac906106799073428c04f058ad83230d2dbe7c94c88f66c3a2ff6e9073e3ab73c1486763cc8a0955a5df2bb0e95a6b285390735359295fd2745a61b95e89755fd9349d5f2d7fde9081906004908101611419565b600060405180830381600087803b15801561069357600080fd5b505af19250505080156106a4575060015b6106e55760405162461bcd60e51b81526020600482015260126024820152713ab83230ba32a632bb32b6189032b93937b960711b6044820152606401610459565b600054604051633e7098eb60e21b81526001600160a01b039091169063f9c263ac906107599073428c04f058ad83230d2dbe7c94c88f66c3a2ff6e9073e3ab73c1486763cc8a0955a5df2bb0e95a6b285390735359295fd2745a61b95e89755fd9349d5f2d7fde9081906004908101611419565b600060405180830381600087803b15801561077357600080fd5b505af1925050508015610784575060015b6107c55760405162461bcd60e51b81526020600482015260126024820152713ab83230ba32a632bb32b6191032b93937b960711b6044820152606401610459565b600054604051633e7098eb60e21b81526001600160a01b039091169063f9c263ac906108399073428c04f058ad83230d2dbe7c94c88f66c3a2ff6e9073e3ab73c1486763cc8a0955a5df2bb0e95a6b285390735359295fd2745a61b95e89755fd9349d5f2d7fde9081906004908101611419565b600060405180830381600087803b15801561085357600080fd5b505af1925050508015610864575060015b6108a55760405162461bcd60e51b81526020600482015260126024820152713ab83230ba32a632bb32b6199032b93937b960711b6044820152606401610459565b600054604051633e7098eb60e21b81526001600160a01b039091169063f9c263ac906109199073428c04f058ad83230d2dbe7c94c88f66c3a2ff6e9073e3ab73c1486763cc8a0955a5df2bb0e95a6b285390735359295fd2745a61b95e89755fd9349d5f2d7fde9081906004908101611419565b600060405180830381600087803b15801561093357600080fd5b505af1925050508015610944575060015b6109855760405162461bcd60e51b81526020600482015260126024820152713ab83230ba32a632bb32b61a1032b93937b960711b6044820152606401610459565b600054604051633e7098eb60e21b81526001600160a01b039091169063f9c263ac906109f99073428c04f058ad83230d2dbe7c94c88f66c3a2ff6e9073e3ab73c1486763cc8a0955a5df2bb0e95a6b285390735359295fd2745a61b95e89755fd9349d5f2d7fde9081906004908101611419565b600060405180830381600087803b158015610a1357600080fd5b505af1925050508015610a24575060015b610a655760405162461bcd60e51b81526020600482015260126024820152713ab83230ba32a632bb32b61a9032b93937b960711b6044820152606401610459565b600054600554604051636cb504a560e11b81526001600160a01b039092169163d96a094a91610a9a9160040190815260200190565b600060405180830381600087803b158015610ab457600080fd5b505af1925050508015610ac5575060015b610afd5760405162461bcd60e51b8152602060048201526009602482015268313abc9032b93937b960b91b6044820152606401610459565b604080516002808252606082018352600092602083019080368337505060015482519293506001600160a01b031691839150600090610b3e57610b3e6114d9565b60200260200101906001600160a01b031690816001600160a01b03168152505073240d7adf8c34fe3908155b8ce4a0e5e74f8dea7e81600181518110610b8657610b866114d9565b6001600160a01b0392831660209182029290920101526003546001546040516370a0823160e01b815230600482015291831692635c11d795929116906370a0823190602401602060405180830381865afa158015610be8573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610c0c91906114ef565b60008473834756c83a0476f4cb9971e534ce909a39b54883610c2f426064611508565b6040518663ffffffff1660e01b8152600401610c4f95949392919061151b565b600060405180830381600087803b158015610c6957600080fd5b505af1925050508015610c7a575060015b610cb35760405162461bcd60e51b815260206004820152600a60248201526939bbb0b81032b93937b960b11b6044820152606401610459565b6000546002546040516370a0823160e01b81523060048201526001600160a01b039283169263e4849b329216906370a0823190602401602060405180830381865afa158015610d06573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610d2a91906114ef565b6040518263ffffffff1660e01b8152600401610d4891815260200190565b600060405180830381600087803b158015610d6257600080fd5b505af1925050508015610d73575060015b610dac5760405162461bcd60e51b815260206004820152600a60248201526939b2b6361032b93937b960b11b6044820152606401610459565b6001546040516370a0823160e01b815230600482015283916001600160a01b0316906370a0823190602401602060405180830381865afa158015610df4573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610e1891906114ef565b11610e715760405162461bcd60e51b815260206004820152602360248201527f444f444f466c6173686c6f616e3a20494e53554646494349454e545f42414c416044820152624e434560e81b6064820152608401610459565b6001546040516370a0823160e01b81523060048201526001600160a01b039091169063a9059cbb9073bde53e676265f361f346ffebf761893ff377767d90859084906370a0823190602401602060405180830381865afa158015610ed9573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610efd91906114ef565b610f07919061158e565b6040516001600160e01b031960e085901b1681526001600160a01b03909216600483015260248201526044016020604051808303816000875af1158015610f52573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610f7691906113f7565b5060405163a9059cbb60e01b81526001600160a01b0385811660048301526024820184905284169063a9059cbb906044016020604051808303816000875af1158015610fc6573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250810190610fea91906113f7565b50505050505050505050565b6000815180845260005b8181101561101c57602081850181015186830182015201611000565b506000602082860101526020601f19601f83011685010191505092915050565b60208152600061104f6020830184610ff6565b9392505050565b6001600160a01b038116811461106b57600080fd5b50565b60008060008060006080868803121561108657600080fd5b853561109181611056565b94506020860135935060408601359250606086013567ffffffffffffffff808211156110bc57600080fd5b818801915088601f8301126110d057600080fd5b8135818111156110df57600080fd5b8960208285010111156110f157600080fd5b9699959850939650602001949392505050565b634e487b7160e01b600052604160045260246000fd5b60006020828403121561112c57600080fd5b813567ffffffffffffffff8082111561114457600080fd5b818401915084601f83011261115857600080fd5b81358181111561116a5761116a611104565b604051601f8201601f19908116603f0116810190838211818310171561119257611192611104565b816040528281528760208487010111156111ab57600080fd5b826020860160208301376000928101602001929092525095945050505050565b6000602082840312156111dd57600080fd5b5035919050565b600181811c908216806111f857607f821691505b60208210810361121857634e487b7160e01b600052602260045260246000fd5b50919050565b601f82111561126a576000816000526020600020601f850160051c810160208610156112475750805b601f850160051c820191505b8181101561126657828155600101611253565b5050505b505050565b815167ffffffffffffffff81111561128957611289611104565b61129d8161129784546111e4565b8461121e565b602080601f8311600181146112d257600084156112ba5750858301515b600019600386901b1c1916600185901b178555611266565b600085815260208120601f198616915b82811015611301578886015182559484019460019091019084016112e2565b508582101561131f5787850151600019600388901b60f8161c191681555b5050505050600190811b01905550565b60006020828403121561134157600080fd5b815161104f81611056565b84815283602082015260018060a01b03831660408201526080606082015260006113796080830184610ff6565b9695505050505050565b60008060006060848603121561139857600080fd5b83356113a381611056565b925060208401356113b381611056565b929592945050506040919091013590565b634e487b7160e01b600052601160045260246000fd5b80820281158282048414176113f1576113f16113c4565b92915050565b60006020828403121561140957600080fd5b8151801515811461104f57600080fd5b6001600160a01b0386811682528581166020808401919091528582166040840152908416606083015260a0608083015282546000918291611459816111e4565b8060a087015260c060018084166000811461147b5760018114611497576114c7565b60ff19851660c08a015260c084151560051b8a010196506114c7565b89600052602060002060005b858110156114be5781548b82018601529083019087016114a3565b8a0160c0019750505b50949c9b505050505050505050505050565b634e487b7160e01b600052603260045260246000fd5b60006020828403121561150157600080fd5b5051919050565b808201808211156113f1576113f16113c4565b600060a08201878352602087602085015260a0604085015281875180845260c08601915060208901935060005b8181101561156d5784516001600160a01b031683529383019391830191600101611548565b50506001600160a01b03969096166060850152505050608001529392505050565b818103818111156113f1576113f16113c456fea2646970667358221220eb82780bf31f38d14f7bd501cfed6805077c67aec91628fe367ec4ac787f772564736f6c63430008160033")
	if err != nil {
		logrus.Fatal(err)
	}
	tx := model.SkyEyeTransaction{
		Chain:           "ethereum",
		BlockNumber:     29065040,
		BlockTimestamp:  1686658180,
		Score:           42,
		Push20Args:      addrLabels,
		TxHash:          "0x037522e093aeb89104f1dcdf8bb1dcfeb6c001617c3515bed66d5a566a3aa52b",
		FromAddress:     "0x3d4609330e3d9df2ea7b5d87e9f5283ec98f13dd",
		ContractAddress: "0x58e3b3ac35351d3f3a51e7d63216a279662377e0",
		Push4Args:       funcs,
		Fund:            "2-Binance: Hot Wallet 10",
		SplitScores:     "0,12,20,50,2,0",
		ByteCode:        data,
	}
	nte := e.(*SkyEyeExporter)
	if err = nte.SendMessageToSlack(tx); err != nil {
		logrus.Fatal(err)
	}
}

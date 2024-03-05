package policy

import (
	"go-etl/model"
	"go-etl/utils"
)

type ContractTypePolicyCalc struct {
	Push4Codes     []string
	Push20Codes    []string
	PushStringLogs []string
}

const ContractTypePolicyName = "ContractType"

func (cpc *ContractTypePolicyCalc) Calc(tx *model.SkyEyeTransaction) int {
	tx.Push4Args = GetPush4Args(cpc.Push4Codes)
	tx.Push20Args = GetPush20Args(tx.Chain, cpc.Push20Codes)
	tx.PushStringLogs = cpc.PushStringLogs
	return 20
}
func (cpc *ContractTypePolicyCalc) Name() string {
	return ContractTypePolicyName
}

func (cpc *ContractTypePolicyCalc) Filter(tx *model.SkyEyeTransaction) bool {
	opCodeArgs := GetPushTypeArgs(tx.ByteCode)
	push4Codes := opCodeArgs[utils.PUSH4]

	if utils.IsToken(utils.Erc20Signatures, push4Codes, utils.Erc20SignatureThreshold) ||
		utils.IsToken(utils.Erc721Signatures, push4Codes, utils.Erc721SignatureThreshold) ||
		utils.IsToken(utils.Erc1155Signatures, push4Codes, utils.Erc1155SignatureThreshold) {
		return true
	}
	cpc.Push4Codes = push4Codes
	cpc.Push20Codes = opCodeArgs[utils.PUSH20]
	cpc.PushStringLogs = opCodeArgs[utils.LOGS]
	return false
}

package model

import (
	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/utils"
)

type ContractTypePolicyCalc struct {
	Push4Codes     []string
	Push20Codes    []string
	PushStringLogs []string
}

const ContractTypePolicyName = "ContractType"

func (cpc *ContractTypePolicyCalc) Calc(tx *SkyEyeTransaction) int {
	tx.Push4Args = GetPush4Args(cpc.Push4Codes)
	tx.Push20Args = GetPush20Args(config.Conf.ETL.Chain, cpc.Push20Codes)
	tx.PushStringLogs = cpc.PushStringLogs
	return 20
}

func (cpc *ContractTypePolicyCalc) Name() string {
	return ContractTypePolicyName
}

func (cpc *ContractTypePolicyCalc) Filter(tx *SkyEyeTransaction) bool {
	opCodeArgs := GetPushTypeArgs(tx.ByteCode)
	funcSignatures := opCodeArgs[utils.PUSH4]

	if utils.IsSkipContract(funcSignatures) {
		return true
	}

	cpc.Push4Codes = funcSignatures
	cpc.Push20Codes = opCodeArgs[utils.PUSH20]
	cpc.PushStringLogs = opCodeArgs[utils.LOGS]
	return false
}

package policy

import "go-etl/model"

type ByteCodePolicyCalc struct{}

func (bpc *ByteCodePolicyCalc) Calc(tx *model.SkyEyeTransaction) int {
	if len(tx.ByteCode) == 0 || len(tx.ByteCode[2:]) < 500 {
		return 0
	}
	return 12
}

func (bpc *ByteCodePolicyCalc) Name() string {
	return "ByteCode"
}

func (bpc *ByteCodePolicyCalc) Filter(tx *model.SkyEyeTransaction) bool {
	return false
}

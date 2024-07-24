package model

type ByteCodePolicyCalc struct{}

func (bpc *ByteCodePolicyCalc) Calc(tx *SkyEyeTransaction) int {
	if len(tx.ByteCode) < 2 || len(tx.ByteCode[2:]) < 500 {
		return 0
	}
	return 12
}

func (bpc *ByteCodePolicyCalc) Name() string {
	return "ByteCode"
}

func (bpc *ByteCodePolicyCalc) Filter(tx *SkyEyeTransaction) bool {
	return false
}

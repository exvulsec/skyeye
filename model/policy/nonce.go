package policy

import "go-etl/model"

type NoncePolicyCalc struct{}

func (npc *NoncePolicyCalc) Calc(tx *model.SkyEyeTransaction) int {
	if tx.Nonce >= 50 {
		return 0
	}
	if 10 <= tx.Nonce && tx.Nonce < 50 {
		return 5 - (int(tx.Nonce)-10)/10
	}
	return 10 - int(tx.Nonce)
}

func (npc *NoncePolicyCalc) Name() string {
	return "Nonce"
}

func (npc *NoncePolicyCalc) Filter(tx *model.SkyEyeTransaction) bool {
	return false
}

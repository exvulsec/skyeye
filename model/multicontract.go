package model

import "strings"

type MultiContractCalc struct{}

func (mcc *MultiContractCalc) Calc(tx *SkyEyeTransaction) int {
	tx.MultiContracts = tx.Trace.ListContracts()
	tx.MultiContractString = strings.Join(tx.MultiContracts, ",")
	if len(tx.MultiContracts) > 1 {
		return 60
	}
	return 0
}

func (mcc *MultiContractCalc) Name() string {
	return "MultiContract"
}

func (mcc *MultiContractCalc) Filter(tx *SkyEyeTransaction) bool {
	return false
}

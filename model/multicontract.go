package model

import "strings"

type MultiContractCalc struct{}

func (mcc *MultiContractCalc) Calc(tx *SkyEyeTransaction) int {
	if len(tx.MultiContracts) > 1 {
		return 60
	}
	return 0
}

func (mcc *MultiContractCalc) Name() string {
	return "MultiContract"
}

func (mcc *MultiContractCalc) Filter(tx *SkyEyeTransaction) bool {
	contracts, skip := tx.Trace.ListContracts()
	if skip {
		return true
	}
	tx.MultiContracts = contracts
	tx.MultiContractString = strings.Join(tx.MultiContracts, ",")
	return false
}

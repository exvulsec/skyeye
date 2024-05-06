package model

import "strings"

type Push4PolicyCalc struct {
	FlashLoanFuncNames []string
}

func (p4pc *Push4PolicyCalc) Calc(tx *SkyEyeTransaction) int {
	score := 0
	if tx.HasFlashLoan(p4pc.FlashLoanFuncNames) {
		score += 30
	}
	if tx.HasStart() {
		score += 30
	}
	return score
}

func (p4pc *Push4PolicyCalc) Name() string {
	return "Push4"
}

func (p4pc *Push4PolicyCalc) Filter(tx *SkyEyeTransaction) bool {
	filterFuncNames := []string{
		"UPGRADE_INTERFACE_VERSION",
		"changeProxyAdmin",
		"changeAdmin",
		"admin",
	}

	for _, funcName := range tx.Push4Args {
		for _, filterFuncName := range filterFuncNames {
			if strings.EqualFold(funcName, filterFuncName) {
				return true
			}
		}
	}

	return false
}

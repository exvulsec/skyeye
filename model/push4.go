package model

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
	return false
}

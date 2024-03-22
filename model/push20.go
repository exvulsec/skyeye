package model

type Push20PolicyCalc struct{}

func (p20pc *Push20PolicyCalc) Calc(tx *SkyEyeTransaction) int {
	if len(tx.Push20Args) == 0 {
		return 0
	}
	if tx.HasRiskAddress([]string{"PancakeSwap: Router v2"}) {
		return 10
	}
	return 5
}

func (p20pc *Push20PolicyCalc) Name() string {
	return "Push20"
}

func (p20pc *Push20PolicyCalc) Filter(tx *SkyEyeTransaction) bool {
	return false
}

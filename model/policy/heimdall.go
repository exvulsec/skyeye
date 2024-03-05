package policy

import (
	"github.com/sirupsen/logrus"

	"go-etl/model"
)

type HeimdallPolicyCalc struct {
	Heimdall model.Heimdall
}

func (hdpc *HeimdallPolicyCalc) Calc(tx *model.SkyEyeTransaction) int {
	if hdpc.GetPolicy(tx) {
		return 30
	}
	return 0
}
func (hdpc *HeimdallPolicyCalc) Name() string {
	return "Heimdall"
}

func (hdpc *HeimdallPolicyCalc) GetPolicy(tx *model.SkyEyeTransaction) bool {

	for _, metadata := range hdpc.Heimdall.MetaData {
		if metadata.View {
			for _, statement := range metadata.ControlStatements {
				if statement == "if (msg.sender == (address(storage[0]))) { .. }" {
					return true
				}
			}
		}
	}
	return false
}

func (hdpc *HeimdallPolicyCalc) Filter(tx *model.SkyEyeTransaction) bool {
	hdl := model.Heimdall{}
	if err := hdl.Get(tx.ContractAddress, tx.ByteCode); err != nil {
		logrus.Error(err)
		return true
	}

	if hdl.FunctionCount > 10 {
		return true
	}

	hdpc.Heimdall = hdl
	return false
}

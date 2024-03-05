package policy

import (
	"github.com/sirupsen/logrus"

	"go-etl/model"
)

type MultiContractCalc struct{}

func (mcc *MultiContractCalc) Calc(tx *model.SkyEyeTransaction) int {
	txTraces := model.GetTransactionTrace(tx.TxHash)
	contractAddrs := []string{}
	for _, txTrace := range txTraces {
		switch txTrace.To {
		case "":
			contractAddrs = append(contractAddrs, txTrace.ContractAddress)
		default:
			if IsInContractAddrs(contractAddrs, txTrace.To) && IsInContractAddrs(contractAddrs, txTrace.From) && txTrace.Input != "0x" {
				tx.IsMultiContract = true
				input := txTrace.Input[:10]
				s := model.Signature{
					ByteSign: input,
				}
				if err := s.GetTextSign(); err != nil {
					logrus.Error(err)
					return 0
				}
				if s.TextSign != "" {
					return 0
				}
				return 60
			}
		}
	}
	return 0
}

func IsInContractAddrs(contracts []string, toAddr string) bool {
	for _, contract := range contracts {
		if contract == toAddr {
			return true
		}
	}
	return false
}

func (mcc *MultiContractCalc) Name() string {
	return "MultiContract"
}

func (mcc *MultiContractCalc) Filter(tx *model.SkyEyeTransaction) bool {
	return false
}

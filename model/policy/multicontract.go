package policy

import (
	"strings"

	"github.com/sirupsen/logrus"

	"go-etl/model"
)

type MultiContractCalc struct{}

func (mcc *MultiContractCalc) Calc(tx *model.SkyEyeTransaction) int {
	multiContracts := []string{}
	txTraces := model.GetTransactionTrace(tx.TxHash)
	for _, txTrace := range txTraces {
		switch txTrace.To {
		case "":
			multiContracts = append(multiContracts, txTrace.ContractAddress)
		default:
			if IsInContractAddrs(multiContracts, txTrace.From) && IsInContractAddrs(multiContracts, txTrace.To) && txTrace.Input != "0x" {
				input := txTrace.Input[:10]
				s := model.Signature{
					ByteSign: input,
				}
				if err := s.GetTextSign(); err != nil {
					logrus.Error(err)
					continue
				}
				if s.TextSign != "" {
					for index, contract := range multiContracts {
						if strings.EqualFold(contract, txTrace.To) {
							multiContracts = append(multiContracts[:index], multiContracts[index+1:]...)
						}
					}
				}
			}
		}
	}
	tx.MultiContracts = multiContracts
	if len(multiContracts) > 1 {
		return 60
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

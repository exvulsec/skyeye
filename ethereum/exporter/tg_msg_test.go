package exporter

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"go-etl/config"
	"go-etl/model"
)

func TestNastiffTransactionExporter_SendToTelegram(t *testing.T) {
	config.SetupConfig("../../config/config.dev.yaml")
	e := NewNastiffTransferExporter("ethereum", "http://47.243.70.228:8088", 0)
	addrLabelString := "Binance_0xe9e7,JetswapFactory,JetswapRouter,PancakeSwap: Router v2,StrategyWingsLP,StrategyWingsLP, 0x{2}"
	addrLabels := strings.Split(addrLabelString, ",")
	funcString := "accumulatedRewardPerShare,addMinterShare,claimReward,collection,owner,pendingReward,renounceOwnership"
	funcs := strings.Split(funcString, ",")
	tx := model.NastiffTransaction{
		Chain:           "ethereum",
		BlockNumber:     29065040,
		BlockTimestamp:  1686658180,
		Score:           42,
		Push20Args:      addrLabels,
		TxHash:          "0x037522e093aeb89104f1dcdf8bb1dcfeb6c001617c3515bed66d5a566a3aa52b",
		FromAddress:     "0x3d4609330e3d9df2ea7b5d87e9f5283ec98f13dd",
		ContractAddress: "0x58e3b3ac35351d3f3a51e7d63216a279662377e0",
		Push4Args:       funcs,
		Fund:            "2-Binance: Hot Wallet 10",
		SplitScores:     "0,12,20,50,2,0",
		ByteCode:        make([]byte, 4277),
	}
	nte := e.(*NastiffTransactionExporter)
	if err := nte.SendMessageToSlack(tx); err != nil {
		logrus.Fatal(err)
	}
}

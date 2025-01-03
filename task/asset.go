package task

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/notifier"
	"github.com/exvulsec/skyeye/utils"
)

type assetTask struct {
	monitorAddresses *model.SkyMonitorAddrs
	notifiers        []notifier.Notifier
}

func NewAssetTask(monitorAddrs *model.SkyMonitorAddrs) Task {
	return &assetTask{
		monitorAddresses: monitorAddrs,
		notifiers:        []notifier.Notifier{notifier.NewLarkNotifier(config.Conf.ETL.LarkAssetWebHook)},
	}
}

func (at *assetTask) Run(data any) any {
	txs, ok := data.(model.Transactions)
	if !ok || len(txs) == 0 {
		return nil
	}
	return at.AnalysisAssetTransfer(txs)
}

func (at *assetTask) AnalysisAssetTransfer(txs model.Transactions) model.Transactions {
	startTime := time.Now()
	conditionFunc := func(tx model.Transaction) bool {
		if tx.ToAddress != nil {
			return at.monitorAddresses.Existed([]string{*tx.ToAddress})
		}
		if tx.MultiContracts != nil {
			return at.monitorAddresses.Existed(tx.MultiContracts)
		}
		return false
	}

	originTxs, needAnalysisTxs := txs.MultiProcess(conditionFunc)
	if len(needAnalysisTxs) > 0 {
		for _, tx := range needAnalysisTxs {
			if tx.Trace != nil {
				tx.IsConstructor = true
			}
			at.ComposeAssetsAndAlert(tx)
		}
		logrus.Infof("block: %d, analysis transactions: %d asset transfer, elapsed: %s",
			needAnalysisTxs[0].BlockNumber, len(needAnalysisTxs), utils.ElapsedTime(startTime))
	}

	return append(originTxs, needAnalysisTxs...)
}

func (at *assetTask) ComposeAssetsAndAlert(tx model.Transaction) {
	assets := model.Assets{
		BlockNumber:    tx.BlockNumber,
		BlockTimestamp: tx.BlockTimestamp,
		TxHash:         tx.TxHash,
		Items:          []model.Asset{},
	}

	if tx.Trace == nil {
		tx.GetTrace(config.Conf.ETL.Chain)
	}
	if tx.Trace != nil {
		skyTx := model.SkyEyeTransaction{Input: tx.Input}
		if tx.ToAddress != nil {
			assets.ToAddress = *tx.ToAddress
		}

		if tx.MultiContracts == nil {
			if err := skyTx.GetInfoByContract(config.Conf.ETL.Chain, *tx.ToAddress); err != nil {
				logrus.Errorf("get skyeye tx info is err %v", err)
			}
			if skyTx.ID == nil {
				return
			}
			if skyTx.MultiContractString != "" && skyTx.MultiContractString != *tx.ToAddress {
				skyTx.MultiContracts = strings.Split(skyTx.MultiContractString, ",")
			}
		} else {
			skyTx.TxHash = tx.TxHash
			skyTx.SplitScores = tx.SplitScores
			skyTx.IsConstructor = tx.IsConstructor
			skyTx.MultiContracts = tx.MultiContracts
		}

		assetTransfers := tx.Trace.ListTransferEventWithDFS(model.AssetTransfers{}, tx.TxHash)
		if err := assets.AnalysisAssetTransfers(assetTransfers); err != nil {
			logrus.Errorf("analysis asset transfer is err: %v", err)
			return
		}
		if len(assets.Items) > 0 {
			at.Alert(assets, skyTx, len(assetTransfers))
		}
	}
}

func (at *assetTask) Alert(assets model.Assets, tx model.SkyEyeTransaction, transferCount int) {
	alertAssets := []model.Asset{}
	threshold, _ := decimal.NewFromString(config.Conf.ETL.AssetUSDAlertThreshold)
	for _, asset := range assets.Items {
		if asset.TotalUSD.Cmp(threshold) >= 0 {
			alertAssets = append(alertAssets, asset)
		}
	}

	if len(alertAssets) > 0 {
		stTime := time.Now()
		assets.Items = alertAssets

		title := fmt.Sprintf("%s %d Malicious Asset Transfer", strings.ToUpper(config.Conf.ETL.Chain), transferCount)
		if at.notifiers != nil {
			for _, n := range at.notifiers {
				n.Notify(at.ComposeLarkNotifierData(tx, title, transferCount, assets))
			}
		}

		logrus.Infof("send asset alert message to lark channel, elapsed: %s", utils.ElapsedTime(stTime))
	}
}

func (at *assetTask) ComposeLarkNotifierData(st model.SkyEyeTransaction, title string, transferCount int, assets model.Assets) notifier.LarkCard {
	return notifier.LarkCard{
		Title:      title,
		TitleColor: "orange",
		ColumnSets: at.ComposeLarkColumnSets(st, transferCount, assets),
		Actions:    at.ComposeLarkActions(assets.TxHash),
	}
}

func (at *assetTask) ComposeLarkColumnSets(st model.SkyEyeTransaction, transferCount int, assets model.Assets) []notifier.LarkColumnSet {
	chain := strings.ToUpper(config.Conf.ETL.Chain)
	scanURL := utils.GetScanURL(chain)
	larkColumnSets := []notifier.LarkColumnSet{
		{
			Columns: []notifier.LarkColumn{
				{Name: "🔗 Chain Name", Value: chain, Weight: 2},
				{Name: "🕐 Time", Value: time.Unix(assets.BlockTimestamp, 0).Format(time.DateTime), Weight: 2},
				{Name: "📦 Block", Value: fmt.Sprintf("%d", assets.BlockNumber), Weight: 1},
			},
		},
		{
			Columns: []notifier.LarkColumn{
				{Name: "#️⃣ Transaction Hash", Value: fmt.Sprintf("[%s](%s)", assets.TxHash, fmt.Sprintf("%s/tx/%s", scanURL, assets.TxHash)), Weight: 1},
			},
		},
		{
			Columns: []notifier.LarkColumn{
				{Name: "📜 Contract", Value: fmt.Sprintf("[%s](%s)", assets.ToAddress, fmt.Sprintf("%s/address/%s", scanURL, assets.ToAddress)), Weight: 4},
			},
		},
		{
			Columns: []notifier.LarkColumn{
				{Name: "🔢 TransferCount", Value: fmt.Sprintf("%d", transferCount), Weight: 1},
				{Name: "🏗️ IsConstructor", Value: fmt.Sprintf("%t", st.IsConstructor), Weight: 1},
			},
		},
	}
	larkColumnSets = append(larkColumnSets, notifier.LarkColumnSet{Name: "HR"})
	for _, asset := range assets.Items {
		larkColumnSets = append(larkColumnSets, notifier.LarkColumnSet{Columns: []notifier.LarkColumn{
			{Name: "🏠 Address", Value: asset.Address, Weight: 2},
			{Name: "💲 TotalUSD", Value: asset.TotalUSD.String(), Weight: 1},
		}})
		for _, token := range asset.Tokens {
			larkColumnSets = append(larkColumnSets, notifier.LarkColumnSet{Columns: []notifier.LarkColumn{
				{Name: "🔶 Token", Value: token.ValueWithUnit, Weight: 2},
				{Name: "💲 TokenUSD", Value: token.ValueUSD.String(), Weight: 1},
			}})
		}
		larkColumnSets = append(larkColumnSets, notifier.LarkColumnSet{Name: "HR"})
	}
	larkColumnSets = append(larkColumnSets, st.ComposeSplitScoresLarkColumnsSet()...)
	return larkColumnSets
}

func (at *assetTask) ComposeLarkActions(txHash string) []notifier.LarkAction {
	larkActions := []notifier.LarkAction{}
	actionURL := ""
	for key, url := range config.Conf.ETL.LinkURLs {
		switch {
		case strings.EqualFold(key, "Phalcon"):
			actionURL = fmt.Sprintf(url, utils.ConvertChainToBlockSecChainID(config.Conf.ETL.Chain), txHash)
		}
		larkActions = append(larkActions, notifier.LarkAction{
			Name: utils.FirstUpper(key),
			URL:  actionURL,
		})
	}
	return larkActions
}

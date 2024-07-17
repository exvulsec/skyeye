package task

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/notifier"
	"github.com/exvulsec/skyeye/utils"
)

type contractTask struct {
	monitorAddresses *model.SkyMonitorAddrs
	notifiers        []notifier.Notifier
}

func NewContractTask(monitorAddrs *model.SkyMonitorAddrs) Task {
	return &contractTask{
		monitorAddresses: monitorAddrs,
		notifiers:        []notifier.Notifier{notifier.NewLarkNotifier(config.Conf.ETL.LarkContractWebHook)},
	}
}

func (ce *contractTask) Run(data any) any {
	txs, ok := data.(model.Transactions)
	if !ok || len(txs) == 0 {
		return nil
	}
	return ce.AnalysisContracts(txs)
}

func (ce *contractTask) AnalysisContracts(txs model.Transactions) model.Transactions {
	startTime := time.Now()
	conditionFunc := func(tx model.Transaction) bool {
		return strings.Contains(tx.Input, "60806040")
	}

	originTxs, needAnalysisTxs := txs.MultiProcess(conditionFunc)

	if len(needAnalysisTxs) > 0 {
		needAnalysisTxs.EnrichTxs()
		for index, tx := range needAnalysisTxs {
			ce.ComposeContractAndAlert(&tx)
			needAnalysisTxs[index] = tx
		}
		logrus.Infof("block: %d, analysis transactions: %d contract creation, elapsed: %s",
			needAnalysisTxs[0].BlockNumber, len(needAnalysisTxs), utils.ElapsedTime(startTime))
	}

	return append(originTxs, needAnalysisTxs...)
}

func (ce *contractTask) ComposeContractAndAlert(tx *model.Transaction) {
	policies := []model.PolicyCalc{
		&model.FundPolicyCalc{Chain: config.Conf.ETL.Chain, NeedFund: true},
		&model.NoncePolicyCalc{},
	}
	skyTx := model.SkyEyeTransaction{}
	skyTx.ConvertFromTransaction(*tx)
	contracts, skip := tx.Trace.ListContracts()
	if skip {
		return
	}
	tx.MultiContracts = contracts
	skyTx.MultiContracts = contracts
	skyTx.MultiContractString = strings.Join(skyTx.MultiContracts, ",")

	for _, p := range policies {
		if p.Filter(&skyTx) {
			return
		}
		score := p.Calc(&skyTx)
		skyTx.Scores = append(skyTx.Scores, fmt.Sprintf("%s: %d", p.Name(), score))
		skyTx.Score += score
	}
	for _, contract := range skyTx.MultiContracts {
		contractTX := model.SkyEyeTransaction{
			Chain:               skyTx.Chain,
			BlockTimestamp:      skyTx.BlockTimestamp,
			BlockNumber:         skyTx.BlockNumber,
			TxHash:              skyTx.TxHash,
			TxPos:               skyTx.TxPos,
			FromAddress:         skyTx.FromAddress,
			ContractAddress:     contract,
			Nonce:               skyTx.Nonce,
			Score:               skyTx.Score,
			Scores:              skyTx.Scores,
			Fund:                skyTx.Fund,
			MonitorAddrs:        ce.monitorAddresses,
			MultiContractString: skyTx.MultiContractString,
		}
		contractTX.Analysis(config.Conf.ETL.Chain)
		if !contractTX.Skip {
			tx.SplitScores = contractTX.SplitScores
			ce.Alert(contractTX)
		}
	}
}

func (ce *contractTask) ComposeLarkNotifierData(st model.SkyEyeTransaction) notifier.LarkCard {
	return notifier.LarkCard{
		Title:      fmt.Sprintf("%s Contract Creation Alert", strings.ToUpper(config.Conf.ETL.Chain)),
		ColumnSets: ce.ComposeLarkColumnSets(st),
		Actions:    ce.ComposeLarkActions(st),
	}
}

func (ce *contractTask) Alert(st model.SkyEyeTransaction) {
	if st.Score > config.Conf.ETL.ScoreAlertThreshold {
		if err := st.SendMessageToSlack(); err != nil {
			logrus.Errorf("send txhash %s's contract %s message to slack is err %v", st.TxHash, st.ContractAddress, err)
		}
		logrus.Infof("monitor contract %s on chain %s", st.ContractAddress, st.Chain)
		if err := st.MonitorContractAddress(); err != nil {
			logrus.Error(err)
			return
		}
		if err := st.Insert(); err != nil {
			logrus.Errorf("insert txhash %s's contract %s to db is err %v", st.TxHash, st.ContractAddress, err)
			return
		}
		if ce.notifiers != nil {
			for _, n := range ce.notifiers {
				switch n.Name() {
				case notifier.LarkNotifierName:
					n.Notify(ce.ComposeLarkNotifierData(st))
				}
			}
		}
	}
}

func (ce *contractTask) ComposeLarkColumnSets(st model.SkyEyeTransaction) []notifier.LarkColumnSet {
	chain := strings.ToUpper(config.Conf.ETL.Chain)
	scanURL := utils.GetScanURL(chain)
	larkColumnSets := []notifier.LarkColumnSet{
		{
			Columns: []notifier.LarkColumn{
				{Name: "🔗 Chain Name", Value: chain, Weight: 2},
				{Name: "🕐 Time", Value: time.Unix(st.BlockTimestamp, 0).Format(time.DateTime), Weight: 2},
				{Name: "📦 Block", Value: fmt.Sprintf("%d", st.BlockNumber), Weight: 1},
			},
		},
		{
			Columns: []notifier.LarkColumn{
				{Name: "💵 Fund", Value: st.Fund, Weight: 2},
				{Name: "🛠️ Functions", Value: strings.Join(st.Push4Args, ","), Weight: 2},
				{Name: "🎯 Score", Value: fmt.Sprintf("%d", st.Score), Weight: 1},
			},
		},
		{
			Columns: []notifier.LarkColumn{
				{Name: "#️⃣ Transaction Hash", Value: fmt.Sprintf("[%s](%s)", st.TxHash, fmt.Sprintf("%s/tx/%s", scanURL, st.TxHash)), Weight: 1},
			},
		},
		{
			Columns: []notifier.LarkColumn{
				{Name: "📜 Contract", Value: fmt.Sprintf("[%s](%s)", st.ContractAddress, fmt.Sprintf("%s/address/%s", scanURL, st.ContractAddress)), Weight: 4},
				{Name: "📝 CodeSize", Value: fmt.Sprintf("%d", len(st.ByteCode)), Weight: 1},
			},
		},
		{
			Columns: []notifier.LarkColumn{
				{Name: "🚀 Deployer", Value: fmt.Sprintf("[%s](%s)", st.FromAddress, fmt.Sprintf("%s/address/%s", scanURL, st.FromAddress)), Weight: 1},
			},
		},
		{
			Columns: []notifier.LarkColumn{
				{Name: "🏷️ Address Labels", Value: strings.Join(st.Push20Args, ","), Weight: 1},
			},
		},
		{
			Columns: []notifier.LarkColumn{
				{Name: "📄 Emit Logs", Value: strings.Join(st.PushStringLogs, ","), Weight: 1},
			},
		},
		{
			Columns: []notifier.LarkColumn{
				{Name: "📊 Split Scores", Value: "", Weight: 1},
			},
		},
	}
	larkColumnSets = append(larkColumnSets, st.ComposeSplitScoresLarkColumnsSet()...)

	return larkColumnSets
}

func (ce *contractTask) ComposeLarkActions(st model.SkyEyeTransaction) []notifier.LarkAction {
	larkActions := []notifier.LarkAction{}
	actionURL := ""
	for key, url := range config.Conf.ETL.LinkURLs {
		switch {
		case strings.EqualFold(key, "Phalcon"):
			actionURL = fmt.Sprintf(url, utils.ConvertChainToBlockSecChainID(config.Conf.ETL.Chain), st.TxHash)
		}
		larkActions = append(larkActions, notifier.LarkAction{
			Name: utils.FirstUpper(key),
			URL:  actionURL,
		})
	}
	return larkActions
}

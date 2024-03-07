package exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"go-etl/client"
	"go-etl/config"
	"go-etl/model"
	"go-etl/model/policy"
	"go-etl/utils"
)

const (
	TransactionContractAddressStream = "evm:contract_address:stream"
	OpenSourceRedisName              = "%s:opensource"
)

type SkyEyeExporter struct {
	items         chan any
	chain         string
	openAPIServer string
	interval      int
	workers       int
	linkURLs      map[string]string
}

func NewSkyEyeExporter(chain, openserver string, interval, workers int) Exporter {
	return &SkyEyeExporter{
		chain:         chain,
		items:         make(chan any, 10),
		openAPIServer: openserver,
		workers:       workers,
		interval:      interval,
		linkURLs: map[string]string{
			"Scan_Contract": fmt.Sprintf("%s/address/%%s", utils.GetScanURL(chain)),
			"Dedaub":        "https://library.dedaub.com/decompile?md5=%s",
			"MCL":           fmt.Sprintf("%s/decompile_%%s/%%s?chain=%s", config.Conf.ETL.MCLServer, chain),
		},
	}
}

func (se *SkyEyeExporter) GetItemsCh() chan any {
	return se.items
}

func (se *SkyEyeExporter) Run() {
	for i := 0; i < se.workers; i++ {
		go se.ExportItems()
	}
}

func (se *SkyEyeExporter) ExportItems() {
	for item := range se.items {
		txs, ok := item.(model.Transactions)
		if !ok {
			continue
		}
		for _, tx := range txs {
			if tx.ToAddress == nil && tx.ContractAddress != "" {
				if tx.TxStatus != 0 {
					go se.exportItem(tx)
				}
			}
		}
	}
}

func (se *SkyEyeExporter) exportItem(tx model.Transaction) {
	policies := []policy.PolicyCalc{
		&policy.MultiContractCalc{},
		&policy.FundPolicyCalc{IsNastiff: true},
		&policy.NoncePolicyCalc{},
	}
	skyTx := model.SkyEyeTransaction{}
	skyTx.ConvertFromTransaction(tx)
	skyTx.Chain = se.chain
	for _, p := range policies {
		if p.Filter(&skyTx) {
			return
		}
		score := p.Calc(&skyTx)
		skyTx.Scores = append(skyTx.Scores, fmt.Sprintf("%s: %d", p.Name(), score))
		skyTx.Score += score
	}
	for _, contract := range skyTx.MultiContracts {
		newSkyTx := model.SkyEyeTransaction{
			Chain:           skyTx.Chain,
			BlockTimestamp:  skyTx.BlockTimestamp,
			BlockNumber:     skyTx.BlockNumber,
			TxHash:          skyTx.TxHash,
			TxPos:           skyTx.TxPos,
			FromAddress:     skyTx.FromAddress,
			ContractAddress: contract,
			Nonce:           skyTx.Nonce,
			Score:           skyTx.Score,
			Scores:          skyTx.Scores,
			Fund:            skyTx.Fund,
		}
		se.processSkyTX(newSkyTx)
	}
}

func (se *SkyEyeExporter) processSkyTX(skyTX model.SkyEyeTransaction) {
	code, err := client.EvmClient().CodeAt(context.Background(), common.HexToAddress(skyTX.ContractAddress), big.NewInt(skyTX.BlockNumber))
	if err != nil {
		logrus.Errorf("get contract %s's bytecode is err %v ", skyTX.ContractAddress, err)
		return
	}
	skyTX.ByteCode = code
	if se.CalcContractByPolicies(&skyTX) {
		return
	}
	skyTX.SplitScores = strings.Join(skyTX.Scores, ",")
	if skyTX.Score > config.Conf.ETL.ScoreAlertThreshold {
		if err = se.SendMessageToSlack(skyTX); err != nil {
			logrus.Errorf("send txhash %s's contract %s message to slack is err %v", skyTX.TxHash, skyTX.ContractAddress, err)
		}
		if skyTX.Score >= config.Conf.ETL.DangerScoreAlertThreshold {
			if err = se.MonitorContractAddress(skyTX); err != nil {
				logrus.Error(err)
			}
		}
	}
	logrus.Infof("start to insert tx %s's contract %s to db", skyTX.TxHash, skyTX.ContractAddress)
	if err = skyTX.Insert(); err != nil {
		logrus.Errorf("insert txhash %s's contract %s to db is err %v", skyTX.TxHash, skyTX.ContractAddress, err)
		return
	}
}

func (se *SkyEyeExporter) CalcContractByPolicies(tx *model.SkyEyeTransaction) bool {
	policies := []policy.PolicyCalc{
		&policy.HeimdallPolicyCalc{},
		&policy.ByteCodePolicyCalc{},
		&policy.ContractTypePolicyCalc{},
		&policy.Push4PolicyCalc{
			FlashLoanFuncNames: policy.LoadFlashLoanFuncNames(),
		},
		&policy.Push20PolicyCalc{},
	}
	for _, p := range policies {
		if p.Filter(tx) {
			return true
		}
		score := p.Calc(tx)
		tx.Scores = append(tx.Scores, fmt.Sprintf("%s: %d", p.Name(), score))
		tx.Score += score
	}
	return false
}

func (se *SkyEyeExporter) MonitorContractAddress(tx model.SkyEyeTransaction) error {
	monitorAddr := model.MonitorAddr{
		Chain:       strings.ToLower(tx.Chain),
		Address:     strings.ToLower(tx.ContractAddress),
		Description: "SkyEye Monitor",
	}
	if err := monitorAddr.Create(); err != nil {
		return fmt.Errorf("create monitor address chain %s address %s is err %v", tx.Chain, tx.ContractAddress, err)
	}
	return nil
}

func (se *SkyEyeExporter) RemoveMonitorContractAddress(tx model.SkyEyeTransaction) error {
	monitorAddr := model.MonitorAddr{
		Chain:   strings.ToLower(tx.Chain),
		Address: strings.ToLower(tx.ContractAddress),
	}
	if err := monitorAddr.Delete(); err != nil {
		return fmt.Errorf("remove monitor address on chain %s address %s is err %v", tx.Chain, tx.ContractAddress, err)
	}
	return nil
}

func (se *SkyEyeExporter) ComposeMessage(tx model.SkyEyeTransaction) string {
	scanURL := utils.GetScanURL(tx.Chain)
	text := fmt.Sprintf("*Chain:* `%s`\n", strings.ToUpper(tx.Chain))
	text += fmt.Sprintf("*Score:* `%d`\n", tx.Score)
	text += fmt.Sprintf("*Funcs:* `%s`\n", strings.Join(tx.Push4Args, ","))
	text += fmt.Sprintf("*Address Labels:* `%s`\n", strings.Join(tx.Push20Args, ","))
	text += fmt.Sprintf("*Emit Logs:* `%s`\n", strings.Join(tx.PushStringLogs, ","))
	text += fmt.Sprintf("*Block:* `%d`\n", tx.BlockNumber)
	text += fmt.Sprintf("*TXhash:* <%s|%s>\n", fmt.Sprintf("%s/tx/%s", scanURL, tx.TxHash), tx.TxHash)
	text += fmt.Sprintf("*DateTime:* `%s UTC`\n", time.Unix(tx.BlockTimestamp, 0).Format(time.DateTime))
	text += fmt.Sprintf("*Contract:* <%s|%s>\n", fmt.Sprintf("%s/address/%s", utils.GetScanURL(tx.Chain), tx.ContractAddress), tx.ContractAddress)
	text += fmt.Sprintf("*Fund:* `%s`\n", tx.Fund)
	text += fmt.Sprintf("*Deployer:* <%s|%s>\n", fmt.Sprintf("%s/address/%s", utils.GetScanURL(tx.Chain), tx.FromAddress), tx.FromAddress)
	text += fmt.Sprintf("*CodeSize:* `%d`\n", len(tx.ByteCode))
	text += fmt.Sprintf("*Split Scores:* `%s`\n", tx.SplitScores)
	return text
}

func (se *SkyEyeExporter) ComposeSlackAction(tx model.SkyEyeTransaction) []slack.AttachmentAction {
	actions := []slack.AttachmentAction{}
	var actionURL string
	for key, url := range se.linkURLs {
		switch key {
		case "Dedaub":
			var dedaubMD5String model.DeDaubResponseString
			err := dedaubMD5String.GetCodeMD5(tx.ByteCode)
			if err != nil {
				logrus.Errorf("get md5 for contract %s is err:", err)
				continue
			}
			actionURL = fmt.Sprintf(url, dedaubMD5String)
		case "Scan_Contract":
			actionURL = fmt.Sprintf(url, tx.ContractAddress)
		case "MCL":
			urlPattern := "contract"
			urlKey := tx.ContractAddress
			actionURL = fmt.Sprintf(url, urlPattern, urlKey)
		}
		actions = append(actions, slack.AttachmentAction{
			Name: key,
			Text: key,
			Type: "button",
			URL:  actionURL,
		})
	}

	return actions
}

func (se *SkyEyeExporter) SendMessageToSlack(tx model.SkyEyeTransaction) error {
	summary := fmt.Sprintf("⚠️Detected a suspected risk transactionon %s, score %d ⚠️\n", strings.ToUpper(se.chain), tx.Score)
	attachment := slack.Attachment{
		Color:      "warning",
		AuthorName: "EXVul",
		Fallback:   summary,
		Text:       summary + se.ComposeMessage(tx),
		Footer:     fmt.Sprintf("skyeye-on-%s", se.chain),
		Ts:         json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
		Actions:    se.ComposeSlackAction(tx),
	}
	msg := slack.WebhookMessage{
		Attachments: []slack.Attachment{attachment},
	}
	return slack.PostWebhook(config.Conf.ETL.SlackWebHook, &msg)
}

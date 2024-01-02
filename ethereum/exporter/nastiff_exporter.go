package exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"go-etl/client"
	"go-etl/config"
	"go-etl/datastore"
	"go-etl/model"
	"go-etl/utils"
)

const (
	TransactionContractAddressStream = "evm:contract_address:stream"
)

type NastiffTransactionExporter struct {
	Chain         string
	OpenAPIServer string
	Interval      int
	BatchSize     int
	LinkURLs      map[string]string
}

func NewNastiffTransferExporter(chain, openserver string, interval, batchSize int) Exporter {
	return &NastiffTransactionExporter{
		Chain:         chain,
		OpenAPIServer: openserver,
		BatchSize:     batchSize,
		LinkURLs: map[string]string{
			"Scan_Contract": fmt.Sprintf("%s/address/%%s", utils.GetScanURL(chain)),
			"Dedaub":        "https://library.dedaub.com/decompile?md5=%s",
		},
	}
}

func (nte *NastiffTransactionExporter) ExportItems(items any) {
	for _, item := range items.(model.Transactions) {
		if item.TxStatus != 0 {
			if item.ToAddress == nil && item.ContractAddress != "" {
				go nte.exportItem(item)
			}
		}
	}
}

func (nte *NastiffTransactionExporter) exportItem(tx model.Transaction) {
	nastiffTX := model.NastiffTransaction{}
	nastiffTX.ConvertFromTransaction(tx)
	nastiffTX.Chain = nte.Chain
	code, err := client.EvmClient().CodeAt(context.Background(), common.HexToAddress(nastiffTX.ContractAddress), nil)
	if err != nil {
		logrus.Errorf("get contract %s's bytecode is err %v ", nastiffTX.ContractAddress, err)
		return
	}
	nastiffTX.ByteCode = code
	nte.CalcContractByPolicies(&nastiffTX)
	if nastiffTX.Score > config.Conf.ETL.ScoreAlertThreshold {
		logrus.Infof("start to insert tx %s's contract %s to redis stream", nastiffTX.TxHash, nastiffTX.ContractAddress)
		if err = nastiffTX.ComposeNastiffValues(); err != nil {
			logrus.Errorf("compose nastiff value by txhash %s's contract %s is err %v", nastiffTX.TxHash, nastiffTX.ContractAddress, err)
			return
		}
		if err = nte.exportToRedis(nastiffTX); err != nil {
			logrus.Errorf("append txhash %s's contract %s to redis message queue is err %v", nastiffTX.TxHash, nastiffTX.ContractAddress, err)
		}
		if err = nte.SendMessageToSlack(nastiffTX); err != nil {
			logrus.Errorf("send txhash %s's contract %s message to slack is err %v", nastiffTX.TxHash, nastiffTX.ContractAddress, err)
		}
		if nastiffTX.Score >= config.Conf.ETL.DangerScoreAlertThreshold {
			if err = nte.MonitorContractAddress(nastiffTX); err != nil {
				logrus.Error(err)
			}
		}
	}
	logrus.Infof("start to insert tx %s's contract %s to db", nastiffTX.TxHash, nastiffTX.ContractAddress)
	if err = nastiffTX.Insert(); err != nil {
		logrus.Errorf("insert txhash %s's contract %s to db is err %v", nastiffTX.TxHash, nastiffTX.ContractAddress, err)
		return
	}
}

func (nte *NastiffTransactionExporter) CalcContractByPolicies(tx *model.NastiffTransaction) {
	policies := []model.PolicyCalc{
		&model.NoncePolicyCalc{},
		&model.ByteCodePolicyCalc{},
		&model.ContractTypePolicyCalc{},
		&model.Push4PolicyCalc{
			FlashLoanFuncNames: model.LoadFlashLoanFuncNames(),
		},
		&model.OpenSourcePolicyCalc{},
		&model.Push20PolicyCalc{},
		&model.FundPolicyCalc{IsNastiff: true, OpenAPIServer: nte.OpenAPIServer},
	}
	splitScores := []string{}
	totalScore := 0
	for _, p := range policies {
		score := p.Calc(tx)
		splitScores = append(splitScores, fmt.Sprintf("%d", score))
		totalScore += score
	}
	tx.SplitScores = strings.Join(splitScores, ",")
	tx.Score = totalScore
}

func (nte *NastiffTransactionExporter) exportToRedis(tx model.NastiffTransaction) error {
	_, err := datastore.Redis().XAdd(context.Background(), &redis.XAddArgs{
		Stream: TransactionContractAddressStream,
		ID:     "*",
		Values: tx.NastiffValues,
	}).Result()
	if err != nil {
		return fmt.Errorf("send values to redis stream is err: %v", err)
	}
	return nil
}

func (nte *NastiffTransactionExporter) MonitorContractAddress(tx model.NastiffTransaction) error {
	monitorAddr := model.MonitorAddr{
		Chain:       strings.ToLower(tx.Chain),
		Address:     strings.ToLower(tx.ContractAddress),
		Description: "Nastiff Monitor",
	}
	if err := monitorAddr.Create(); err != nil {
		return fmt.Errorf("create monitor address chain %s address %s is err %v", tx.Chain, tx.ContractAddress, err)
	}
	return nil
}

func (nte *NastiffTransactionExporter) RemoveMonitorContractAddress(tx model.NastiffTransaction) error {
	monitorAddr := model.MonitorAddr{
		Chain:   strings.ToLower(tx.Chain),
		Address: strings.ToLower(tx.ContractAddress),
	}
	if err := monitorAddr.Delete(); err != nil {
		return fmt.Errorf("remove monitor address on chain %s address %s is err %v", tx.Chain, tx.ContractAddress, err)
	}
	return nil
}

func (nte *NastiffTransactionExporter) ComposeMessage(tx model.NastiffTransaction) string {
	scanURL := utils.GetScanURL(tx.Chain)
	text := fmt.Sprintf("*Chain:* `%s`\n", strings.ToUpper(tx.Chain))
	text += fmt.Sprintf("*Block:* `%d`\n", tx.BlockNumber)
	text += fmt.Sprintf("*TXhash:* <%s|%s>\n", fmt.Sprintf("%s/tx/%s", scanURL, tx.TxHash), tx.TxHash)
	text += fmt.Sprintf("*DateTime:* `%s UTC`\n", time.Unix(tx.BlockTimestamp, 0).Format("2006-01-02 15:04:05"))
	text += fmt.Sprintf("*Contract:* <%s|%s>\n", fmt.Sprintf("%s/address/%s", utils.GetScanURL(tx.Chain), tx.ContractAddress), tx.ContractAddress)
	text += fmt.Sprintf("*Fund:* `%s`\n", tx.Fund)
	text += fmt.Sprintf("*Deployer:* <%s|%s>\n", fmt.Sprintf("%s/address/%s", utils.GetScanURL(tx.Chain), tx.FromAddress), tx.FromAddress)
	text += fmt.Sprintf("*Score:* `%d`\n", tx.Score)
	text += fmt.Sprintf("*Funcs:* `%s`\n", strings.Join(tx.Push4Args, ","))
	text += fmt.Sprintf("*Address Labels:* `%s`\n", strings.Join(tx.Push20Args, ","))
	text += fmt.Sprintf("*Emit Logs:* `%s`\n", strings.Join(tx.PushStringLogs, ","))
	text += fmt.Sprintf("*CodeSize:* `%d`\n", len(tx.ByteCode))
	text += fmt.Sprintf("*Split Scores:* `%s`\n", tx.SplitScores)
	return text
}

func (nte *NastiffTransactionExporter) ComposeSlackAction(tx model.NastiffTransaction) []slack.AttachmentAction {
	actions := []slack.AttachmentAction{}
	var actionURL = ""
	for key, url := range nte.LinkURLs {
		if key == "Dedaub" {
			var dedaubMD5String model.DeDaubResponseString
			err := dedaubMD5String.GetCodeMD5(tx.ByteCode)
			if err != nil {
				logrus.Errorf("get md5 for contract %s is err:", err)
				continue
			}
			logrus.Info(dedaubMD5String)
			actionURL = fmt.Sprintf(url, dedaubMD5String)
		} else {
			actionURL = fmt.Sprintf(url, tx.ContractAddress)
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

func (nte *NastiffTransactionExporter) SendMessageToSlack(tx model.NastiffTransaction) error {
	summary := fmt.Sprintf("⚠️Detected a suspected risk transaction on *%s*⚠️\n", strings.ToUpper(nte.Chain))
	attachment := slack.Attachment{
		Color:      "warning",
		AuthorName: "EXVul",
		Fallback:   summary,
		Text:       summary + nte.ComposeMessage(tx),
		Footer:     fmt.Sprintf("skyeye-on-%s", nte.Chain),
		Ts:         json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
		Actions:    nte.ComposeSlackAction(tx),
	}
	msg := slack.WebhookMessage{
		Attachments: []slack.Attachment{attachment},
	}
	return slack.PostWebhook(config.Conf.ETL.SlackWebHook, &msg)
}

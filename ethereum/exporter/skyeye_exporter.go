package exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
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
		},
	}
}

func (se *SkyEyeExporter) GetItemsCh() chan any {
	return se.items
}

func (se *SkyEyeExporter) Run() {
	go se.watchContractKeysFromRedis()
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
	skyTx := model.SkyEyeTransaction{}
	skyTx.ConvertFromTransaction(tx)
	skyTx.Chain = se.chain
	if err := se.exportToRedis(skyTx); err != nil {
		logrus.Errorf("append txhash %s's contract %s to redis message queue is err %v", skyTx.TxHash, skyTx.ContractAddress, err)
	}
}

func (se *SkyEyeExporter) processKey(contractAddress string) {
	var redisHashName = fmt.Sprintf(OpenSourceRedisName, se.chain)
	var skyTX model.SkyEyeTransaction
	value, err := datastore.Redis().HGet(context.Background(), redisHashName, contractAddress).Result()
	if err != nil {
		logrus.Errorf("error getting timestamp for %s: %v", contractAddress, err)
		return
	}
	err = json.Unmarshal([]byte(value), &skyTX)
	if err != nil {
		logrus.Errorf("unmarshall %s to skyeye tx instance is err: %v", value, err)
		return
	}

	timestampTime, err := time.Parse(time.RFC3339, time.Unix(skyTX.BlockTimestamp, 0).Format(time.RFC3339))
	if err != nil {
		logrus.Errorf("error parsing timestamp for %s: %v", contractAddress, err)
		return
	}

	timeDifference := time.Since(timestampTime)
	if timeDifference < time.Minute*time.Duration(se.interval) {
		return
	}
	se.processSkyTX(skyTX)
}

func (se *SkyEyeExporter) processSkyTX(skyTX model.SkyEyeTransaction) {
	code, err := client.EvmClient().CodeAt(context.Background(), common.HexToAddress(skyTX.ContractAddress), nil)
	if err != nil {
		logrus.Errorf("get contract %s's bytecode is err %v ", skyTX.ContractAddress, err)
		return
	}
	skyTX.ByteCode = code
	se.CalcContractByPolicies(&skyTX)
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
	se.removeKeyFromRedis(skyTX.ContractAddress)
}

func (se *SkyEyeExporter) removeKeyFromRedis(contractAddress string) {
	var redisHashName = fmt.Sprintf(OpenSourceRedisName, se.chain)
	if _, err := datastore.Redis().HDel(context.Background(), redisHashName, contractAddress).Result(); err != nil {
		logrus.Errorf("delete field %s in key %s is err %v", contractAddress, redisHashName, err)
		return
	}
}

func (se *SkyEyeExporter) scanKeysFromRedis() {
	var redisHashName = fmt.Sprintf(OpenSourceRedisName, se.chain)
	keys, err := datastore.Redis().HKeys(context.Background(), redisHashName).Result()
	if err != nil {
		logrus.Errorf("error scanning keys: %v", err)
		return
	}
	for _, key := range keys {
		se.processKey(key)
	}
}

func (se *SkyEyeExporter) watchContractKeysFromRedis() {
	for {
		se.scanKeysFromRedis()
		time.Sleep(1 * time.Minute)
	}
}

func (se *SkyEyeExporter) CalcContractByPolicies(tx *model.SkyEyeTransaction) {
	policies := []model.PolicyCalc{
		&model.NoncePolicyCalc{},
		&model.ByteCodePolicyCalc{},
		&model.ContractTypePolicyCalc{},
		&model.Push4PolicyCalc{
			FlashLoanFuncNames: model.LoadFlashLoanFuncNames(),
		},
		&model.OpenSourcePolicyCalc{},
		&model.Push20PolicyCalc{},
		&model.FundPolicyCalc{IsNastiff: true, OpenAPIServer: se.openAPIServer},
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

func (se *SkyEyeExporter) exportToRedis(tx model.SkyEyeTransaction) error {
	var redisName = fmt.Sprintf(OpenSourceRedisName, se.chain)
	txBytes, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("marhsal the sky eye tx is err %v", err)
	}
	_, err = datastore.Redis().HSet(context.Background(), redisName, tx.ContractAddress, string(txBytes)).Result()
	if err != nil {
		return fmt.Errorf("set nastiff value to redis %s for key %s is err: %v", redisName, tx.ContractAddress, err)
	}
	return nil
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

func (se *SkyEyeExporter) SendMessageToSlack(tx model.SkyEyeTransaction) error {
	summary := fmt.Sprintf("⚠️Detected a suspected risk transaction on *%s*⚠️\n", strings.ToUpper(se.chain))
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

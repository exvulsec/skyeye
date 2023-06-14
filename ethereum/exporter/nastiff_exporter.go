package exporter

import (
	"context"
	"fmt"
	"strings"
	"time"

	goteamsnotify "github.com/atc0005/go-teams-notify/v2"
	"github.com/atc0005/go-teams-notify/v2/messagecard"
	"github.com/ethereum/go-ethereum/common"
	tgbotAPI "github.com/go-telegram/bot"
	tgbotModels "github.com/go-telegram/bot/models"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

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
	Chain            string
	OpenAPIServer    string
	Interval         int
	TeamsClient      *goteamsnotify.TeamsClient
	TGBot            TGBot
	LinkURLs         map[string]string
	OpenSourcePolicy model.OpenSourcePolicy
}

type TGBot struct {
	BoTAPI      *tgbotAPI.Bot
	ChatConfigs []config.TGChatConfig
}

func NewNastiffTransferExporter(chain, openserver string, interval int) Exporter {

	botAPI, err := tgbotAPI.New(config.Conf.ETL.TGBot.Token)
	if err != nil {
		logrus.Panicf("new tg bot api is err: %v", err)
	}
	tgBot := TGBot{
		BoTAPI:      botAPI,
		ChatConfigs: config.Conf.ETL.TGBot.ChatConfigs,
	}

	return &NastiffTransactionExporter{
		Chain:         chain,
		OpenAPIServer: openserver,
		LinkURLs: map[string]string{
			"ScanAddress": fmt.Sprintf("%s/address/%%s", utils.GetScanURL(chain)),
			"ScanTX":      fmt.Sprintf("%s/tx/%%s", utils.GetScanURL(chain)),
			"Dedaub":      fmt.Sprintf("%s/api/v1/address/%%s/dedaub?apikey=%s&chain=%s", openserver, config.Conf.HTTPServer.APIKey, chain),
			"MCL":         fmt.Sprintf("%s/api/v1/address/%%s/solidity?apikey=%s&chain=%s", openserver, config.Conf.HTTPServer.APIKey, chain),
		},
		OpenSourcePolicy: model.OpenSourcePolicy{Interval: interval},
		TeamsClient:      goteamsnotify.NewTeamsClient(),
		TGBot:            tgBot,
	}
}

func (nte *NastiffTransactionExporter) ExportItems(items any) {
	for _, item := range items.(model.Transactions) {
		if item.TxStatus != 0 {
			if item.ToAddress == nil && item.ContractAddress != "" {
				nt := model.NastiffTransaction{}
				nt.ConvertFromTransaction(item)
				nt.Chain = nte.Chain
				code, err := client.EvmClient().CodeAt(context.Background(), common.HexToAddress(nt.ContractAddress), nil)
				if err != nil {
					logrus.Errorf("get contract %s's bytecode is err %v ", nt.ContractAddress, err)
					continue
				}
				nt.ByteCode = code
				go nte.exportItem(nt)
			}
		}
	}
}

func (nte *NastiffTransactionExporter) exportItem(tx model.NastiffTransaction) {
	isFilter := nte.CalcContractByPolicies(&tx)
	if !isFilter {
		logrus.Infof("start to insert tx %s's contract %s to redis stream", tx.TxHash, tx.ContractAddress)
		if err := tx.ComposeNastiffValues(); err != nil {
			logrus.Errorf("compose nastiff value by txhash %s's contract %s is err %v", tx.TxHash, tx.ContractAddress, err)
			return
		}
		if err := nte.exportToRedis(tx); err != nil {
			logrus.Errorf("append txhash %s's contract %s to redis message queue is err %v", tx.TxHash, tx.ContractAddress, err)
		}
		if err := nte.Alert(tx); err != nil {
			logrus.Errorf("alert txhash %s's contract %s to channel is err %v", tx.TxHash, tx.ContractAddress, err)
		}
		if tx.Score >= config.Conf.ETL.DangerScoreAlertThreshold {
			if err := nte.MonitorContractAddress(tx); err != nil {
				logrus.Error(err)
			}
			sentMsgs, err := nte.SendToTelegram(tx)
			if err != nil {
				logrus.Error(err)
			}
			go func() {
				if nte.OpenSourcePolicy.IsOpenSource(tx) {
					if err = nte.RemoveMonitorContractAddress(tx); err != nil {
						logrus.Error(err)
					}
					if err = nte.UpdateTGMessage(sentMsgs); err != nil {
						logrus.Error(err)
					}
				}
			}()
		}
	}
	logrus.Infof("start to insert tx %s's contract %s to db", tx.TxHash, tx.ContractAddress)
	if err := tx.Insert(); err != nil {
		logrus.Errorf("insert txhash %s's contract %s to db is err %v", tx.TxHash, tx.ContractAddress, err)
		return
	}
}

func (nte *NastiffTransactionExporter) UpdateTGMessage(sentMsgs []*tgbotModels.Message) error {
	for _, sentMsg := range sentMsgs {
		editMsgTextParams := &tgbotAPI.EditMessageTextParams{
			ChatID:      sentMsg.Chat.ID,
			MessageID:   sentMsg.ID,
			ParseMode:   tgbotModels.ParseModeHTML,
			Text:        fmt.Sprintf("<strike>%s</strike>", sentMsg.Text),
			ReplyMarkup: sentMsg.ReplyMarkup,
		}
		_, err := nte.TGBot.BoTAPI.EditMessageText(context.Background(), editMsgTextParams)
		if err != nil {
			return fmt.Errorf("edit tg msg %d on chat %d is err %v", sentMsg.ID, sentMsg.Chat.ID, err)
		}
	}
	return nil
}

func (nte *NastiffTransactionExporter) CalcContractByPolicies(tx *model.NastiffTransaction) bool {
	policies := []model.PolicyCalc{
		&model.NoncePolicyCalc{},
		&model.ByteCodePolicyCalc{},
		&model.ContractTypePolicyCalc{},
		&model.Push4PolicyCalc{
			FlashLoanFuncNames: model.LoadFlashLoanFuncNames(),
		},
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
	return tx.Score < config.Conf.ETL.ScoreAlertThreshold
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

func (nte *NastiffTransactionExporter) ComposePotentialActionOpenURI(tx model.NastiffTransaction) []*messagecard.PotentialAction {
	LinkURLKeys := []string{"ScanAddress", "ScanTX", "Dedaub", "MCL"}
	potentialActions := []*messagecard.PotentialAction{}
	for _, linkURLKey := range LinkURLKeys {
		potentialAction, _ := messagecard.NewPotentialAction(messagecard.PotentialActionOpenURIType, linkURLKey)
		linkURL := nte.LinkURLs[linkURLKey]
		url := ""
		if strings.EqualFold(linkURLKey, "ScanTX") {
			url = fmt.Sprintf(linkURL, tx.TxHash)
		} else {
			url = fmt.Sprintf(linkURL, tx.ContractAddress)
		}

		potentialAction.PotentialActionOpenURI = messagecard.PotentialActionOpenURI{Targets: []messagecard.PotentialActionOpenURITarget{{OS: "default", URI: url}}}
		potentialActions = append(potentialActions, potentialAction)
	}
	return potentialActions
}

func (nte *NastiffTransactionExporter) Alert(tx model.NastiffTransaction) error {
	msgCard := messagecard.NewMessageCard()
	msgCard.Title = fmt.Sprintf("%s %d", tx.Chain, tx.Score)
	msgCard.Summary = "got an alert"
	section := messagecard.NewSection()

	facts := []messagecard.SectionFact{}
	for key, value := range tx.NastiffValues {
		if value == "" {
			value = "None"
		}
		facts = append(facts, messagecard.SectionFact{
			Name:  key,
			Value: value,
		})
	}

	if err := section.AddFact(facts...); err != nil {
		return fmt.Errorf("add fact to section is err: %v", err)
	}
	if err := msgCard.AddSection(section); err != nil {
		return fmt.Errorf("add seciton to message card is err: %v", err)
	}
	if err := msgCard.AddPotentialAction(nte.ComposePotentialActionOpenURI(tx)...); err != nil {
		return fmt.Errorf("add potential action to message card is err: %v", err)
	}

	if tx.Score >= config.Conf.ETL.DangerScoreAlertThreshold {
		msgCard.ThemeColor = "#E1395F"
	} else {
		msgCard.ThemeColor = "#1EC6A0"
	}

	if err := nte.TeamsClient.Send(config.Conf.ETL.TeamsAlertWebHook, msgCard); err != nil {
		return fmt.Errorf("send message to channel is err: %v", err)
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

func (nte *NastiffTransactionExporter) ComposeTGMessageMarkUp(tx model.NastiffTransaction) tgbotModels.InlineKeyboardMarkup {
	markup := tgbotModels.InlineKeyboardMarkup{}
	inlineKeyboardButtons := []tgbotModels.InlineKeyboardButton{
		{
			Text: "Dedaub",
			URL: fmt.Sprintf("%s/api/v1/address/%s/dedaub?apikey=%s&chain=%s",
				nte.OpenAPIServer,
				tx.ContractAddress,
				config.Conf.HTTPServer.APIKey,
				tx.Chain,
			)},
	}
	markup.InlineKeyboard = [][]tgbotModels.InlineKeyboardButton{inlineKeyboardButtons}
	return markup
}

func (nte *NastiffTransactionExporter) SendToTelegram(tx model.NastiffTransaction) ([]*tgbotModels.Message, error) {
	sentMsgs := []*tgbotModels.Message{}
	for _, cfg := range nte.TGBot.ChatConfigs {
		msgParams := []*tgbotAPI.SendMessageParams{}
		template := nte.composeTGTemplate(tx, cfg.External)
		chatMsgParams := &tgbotAPI.SendMessageParams{
			ChatID:    cfg.ChatID,
			Text:      template,
			ParseMode: tgbotModels.ParseModeHTML,
		}
		if !cfg.External {
			chatMsgParams.ReplyMarkup = nte.ComposeTGMessageMarkUp(tx)
		}
		if len(cfg.ThreadIDs) > 0 {
			for _, threadID := range cfg.ThreadIDs {
				threadMsgParams := chatMsgParams
				threadMsgParams.MessageThreadID = threadID
				msgParams = append(msgParams, threadMsgParams)
			}
		} else {
			msgParams = append(msgParams, chatMsgParams)
		}
		for _, msgParam := range msgParams {
			messageInfo, err := nte.TGBot.BoTAPI.SendMessage(context.Background(), msgParam)
			if err != nil {
				return sentMsgs, fmt.Errorf("send message to chat id %d is err %v", cfg.ChatID, err)
			}
			sentMsgs = append(sentMsgs, messageInfo)
		}
	}
	return sentMsgs, nil
}

func (nte *NastiffTransactionExporter) composeTGTemplate(tx model.NastiffTransaction, external bool) string {
	scanURL := utils.GetScanURL(tx.Chain)
	if external {
		return fmt.Sprintf(`
<tg-emoji emoji-id="5368324170671202286">‼️</tg-emoji><b> %s Alert</b><tg-emoji emoji-id="5368324170671202286">‼️</tg-emoji>

<b>Chain:</b> %s
<b>Block:</b> %d
<b>TXhash:</b> <a href="%s">%s</a>
<b>DateTime:</b> %s UTC
<b>Contract:</b> <a href="%s">%s</a>
<b>Deployer:</b> <a href="%s">%s</a>
<b>Score:</b> <pre>%d</pre>
<b>Address Labels:</b> %s
`,
			strings.ToUpper(tx.Chain),
			strings.ToUpper(tx.Chain),
			tx.BlockNumber,
			fmt.Sprintf("%s/tx/%s", scanURL, tx.TxHash), tx.TxHash,
			time.Unix(tx.BlockTimestamp, 0).Format("2006-01-02 15:04:05"),
			fmt.Sprintf("%s/address/%s", utils.GetScanURL(tx.Chain), tx.ContractAddress), tx.ContractAddress,
			fmt.Sprintf("%s/address/%s", utils.GetScanURL(tx.Chain), tx.FromAddress), tx.FromAddress,
			tx.Score,
			strings.Join(tx.Push20Args, ","))
	}
	return fmt.Sprintf(`
<tg-emoji emoji-id="5368324170671202286">‼️</tg-emoji><b> %s Alert</b><tg-emoji emoji-id="5368324170671202286">‼️</tg-emoji>

<b>Chain:</b> %s
<b>Block:</b> %d
<b>TXhash:</b> <a href="%s">%s</a>
<b>DateTime:</b> %s UTC
<b>Contract:</b> <a href="%s">%s</a>
<b>CodeSize:</b> %d
<b>Fund:</b> %s
<b>Score:</b> <pre>%d</pre>
<b>Split Scores:</b> %s
<b>Deployer:</b> <a href="%s">%s</a>
<b>Funcs: </b> %s
<b>Address Labels:</b> %s
`,
		strings.ToUpper(tx.Chain),
		strings.ToUpper(tx.Chain),
		tx.BlockNumber,
		fmt.Sprintf("%s/tx/%s", scanURL, tx.TxHash), tx.TxHash,
		time.Unix(tx.BlockTimestamp, 0).Format("2006-01-02 15:04:05"),
		fmt.Sprintf("%s/address/%s", utils.GetScanURL(tx.Chain), tx.ContractAddress), tx.ContractAddress,
		len(tx.ByteCode),
		tx.Fund,
		tx.Score,
		tx.SplitScores,
		fmt.Sprintf("%s/address/%s", utils.GetScanURL(tx.Chain), tx.FromAddress), tx.FromAddress,
		strings.Join(tx.Push4Args, ","),
		strings.Join(tx.Push20Args, ","))
}

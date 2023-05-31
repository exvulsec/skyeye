package exporter

import (
	"context"
	"fmt"
	"strings"

	goteamsnotify "github.com/atc0005/go-teams-notify/v2"
	"github.com/atc0005/go-teams-notify/v2/messagecard"
	"github.com/ethereum/go-ethereum/common"
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
	Chain         string
	Nonce         uint64
	OpenAPIServer string
	Interval      int64
	TeamsClient   *goteamsnotify.TeamsClient
	AlterWebHook  string
	LinkURLs      map[string]string
}

func NewNastiffTransferExporter(chain, openserver, alterWebHook string, nonce uint64, interval int64) Exporter {
	return &NastiffTransactionExporter{
		Chain:         chain,
		Nonce:         nonce,
		OpenAPIServer: openserver,
		Interval:      interval,
		LinkURLs: map[string]string{
			"ScanAddress": fmt.Sprintf("%s/address/%%s", utils.GetScanURL(chain)),
			"ScanTX":      fmt.Sprintf("%s/tx/%%s", utils.GetScanURL(chain)),
			"Dedaub":      fmt.Sprintf("%s/api/v1/address/%%s/dedaub?apikey=%s&chain=%s", openserver, config.Conf.HTTPServer.APIKey, chain),
			"MCL":         fmt.Sprintf("%s/api/v1/address/%%s/solidity?apikey=%s&chain=%s", openserver, config.Conf.HTTPServer.APIKey, chain),
		},
		TeamsClient:  goteamsnotify.NewTeamsClient(),
		AlterWebHook: alterWebHook,
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
			return
		}
		if err := nte.Alert(tx); err != nil {
			logrus.Errorf("alert txhash %s's contract %s to channel is err %v", tx.TxHash, tx.ContractAddress, err)
			return
		}
	}
	logrus.Infof("start to insert tx %s's contract %s to db", tx.TxHash, tx.ContractAddress)
	if err := tx.Insert(); err != nil {
		logrus.Errorf("insert txhash %s's contract %s to db is err %v", tx.TxHash, tx.ContractAddress, err)
		return
	}

}

func (nte *NastiffTransactionExporter) CalcContractByPolicies(tx *model.NastiffTransaction) bool {
	policies := []model.PolicyCalc{
		&model.NoncePolicyCalc{ThresholdNonce: nte.Nonce},
		&model.ByteCodePolicyCalc{},
		&model.ContractTypePolicyCalc{},
		&model.OpenSourcePolicyCalc{Interval: config.Conf.ETL.ScanInterval},
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
	tx.SplitScores = splitScores
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
		if strings.ToLower(linkURLKey) == "scantx" {
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

	if tx.Score >= 90 {
		msgCard.ThemeColor = "#E1395F"
	} else {
		msgCard.ThemeColor = "#1EC6A0"
	}

	if err := msgCard.AddPotentialAction(nte.ComposePotentialActionOpenURI(tx)...); err != nil {
		return fmt.Errorf("add potential action to message card is err: %v", err)
	}

	if err := nte.TeamsClient.Send(nte.AlterWebHook, msgCard); err != nil {
		return fmt.Errorf("send message to channel is err: %v", err)
	}
	return nil
}

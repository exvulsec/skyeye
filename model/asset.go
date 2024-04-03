package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/utils"
)

const (
	TransferTopic        = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	WithdrawalTopic      = "0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65"
	DepositTopic         = "0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c"
	TransferABIName      = "Transfer"
	TransferIndexABIName = "TransferIndex"
	WithdrawalABIName    = "Withdrawal"
	DepositABIName       = "Deposit"

	ABIs = `[
		{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":true,"name":"value","type":"uint256"}],"name":"TransferIndex","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"name":"src","type":"address"},{"indexed":false,"name":"wad","type":"uint256"}],"name":"Withdrawal","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"name":"dst","type":"address"},{"indexed":false,"name":"wad","type":"uint256"}],"name":"Deposit","type":"event"}
	]`
)

type AssetBalances map[string]map[string]decimal.Decimal

type Asset struct {
	Address  string          `json:"address"`
	Tokens   []Token         `json:"assets"`
	TotalUSD decimal.Decimal `json:"total_usd"`
}

type Event map[string]any

type Assets struct {
	BlockNumber    int64
	BlockTimestamp int64
	TxHash         string
	ToAddress      string
	Items          []Asset
	TotalUSD       decimal.Decimal
}

type AssetTransfer struct {
	From    string
	To      string
	Value   decimal.Decimal
	Address string
}

type AssetTransfers []AssetTransfer

var Threshold decimal.Decimal

func init() {
	var err error
	Threshold, err = decimal.NewFromString(config.Conf.ETL.AssetUSDAlertThreshold)
	if err != nil {
		logrus.Panicf("convert asset usd alert threshold is err %v", err)
	}
}

func (ats *AssetTransfers) compose(logs []*types.Log, trace TransactionTrace) {
	for _, l := range logs {
		assetTransfer := AssetTransfer{}
		err := assetTransfer.Decode(*l)
		if err != nil {
			logrus.Error(err)
			return
		}
		if assetTransfer.Address != "" {
			*ats = append(*ats, assetTransfer)
		}
	}
	*ats = append(*ats, trace.ListTransferEvent()...)
}

func (a *AssetTransfer) Decode(log types.Log) error {
	eventAbi, err := abi.JSON(strings.NewReader(ABIs))
	if err != nil {
		return err
	}
	topic := log.Topics[0].String()

	abiName := decodeWithTopic(topic)
	if abiName == "" {
		return nil
	}
	if abiName == TransferABIName {
		if len(log.Topics) == 4 {
			abiName = TransferIndexABIName
		}
	}

	event := map[string]interface{}{}
	err = eventAbi.UnpackIntoMap(event, abiName, log.Data)
	if err != nil {
		return errors.New("unpack abi is err: " + err.Error() + "on tx: " + log.TxHash.String())
	}
	a.DecodeEvent(topic, event, log)

	return nil
}

func decodeWithTopic(topic string) string {
	switch strings.ToLower(topic) {
	case TransferTopic:
		return TransferABIName
	case WithdrawalTopic:
		return WithdrawalABIName
	case DepositTopic:
		return DepositABIName
	}
	return ""
}

func (a *AssetTransfer) DecodeEvent(topic string, event map[string]any, log types.Log) {
	switch strings.ToLower(topic) {
	case TransferTopic:
		a.DecodeTransfer(event, log)
	case WithdrawalTopic:
		a.DecodeWithdrawal(event, log)
	case DepositTopic:
		a.DecodeDeposit(event, log)
	}
}

func (a *AssetTransfer) DecodeTransfer(event Event, log types.Log) {
	if event.mapKeyExist("from") && event.mapKeyExist("to") {
		a.From = convertAddress(event["from"].(string))
		a.To = convertAddress(event["to"].(string))
	} else {
		a.From = convertAddress(log.Topics[1].String())
		a.To = convertAddress(log.Topics[2].String())
	}
	if event.mapKeyExist("value") {
		a.Value = decimal.NewFromBigInt(event["value"].(*big.Int), 0)
	} else {
		value, _ := (&big.Int{}).SetString(utils.RemoveLeadingZeroDigits(log.Topics[3].String()), 16)
		a.Value = decimal.NewFromBigInt(value, 0)
	}

	a.Address = strings.ToLower(log.Address.String())
}

func (a *AssetTransfer) DecodeWithdrawal(event Event, log types.Log) {
	if event.mapKeyExist("src") {
		a.From = convertAddress(event["src"].(string))
	} else {
		a.From = convertAddress(log.Topics[1].String())
	}
	a.Value = decimal.NewFromBigInt(event["wad"].(*big.Int), 0)
	a.Address = strings.ToLower(log.Address.String())
}

func (a *AssetTransfer) DecodeDeposit(event Event, log types.Log) {
	if event.mapKeyExist("dst") {
		a.To = convertAddress(event["dst"].(string))
	} else {
		a.To = convertAddress(log.Topics[1].String())
	}
	a.Value = decimal.NewFromBigInt(event["wad"].(*big.Int), 0)
	a.Address = strings.ToLower(log.Address.String())
}

func (abs *AssetBalances) SetBalanceValue(address, token string, value decimal.Decimal) {
	if address == "" {
		return
	}
	_, ok := (*abs)[address]
	if !ok {
		(*abs)[address] = map[string]decimal.Decimal{}
	}
	_, ok = (*abs)[address][token]
	if !ok {
		(*abs)[address][token] = value
	} else {
		(*abs)[address][token] = (*abs)[address][token].Add(value)
	}
}

func (abs *AssetBalances) calcBalance(transfers []AssetTransfer, focuses []string) {
	for _, transfer := range transfers {
		if !transfer.Value.Equal(decimal.Decimal{}) {
			abs.SetBalanceValue(transfer.From, transfer.Address, transfer.Value.Mul(decimal.NewFromInt(-1)))
			abs.SetBalanceValue(transfer.To, transfer.Address, transfer.Value)
		}
	}
	abs.filterBalance(focuses)
}

func checkAddressExisted(focuses []string, address string) bool {
	for _, focus := range focuses {
		if strings.EqualFold(address, focus) {
			return true
		}
	}
	return false
}

func (abs *AssetBalances) filterBalance(focuses []string) {
	for address := range *abs {
		if !checkAddressExisted(focuses, address) {
			delete(*abs, address)
		}
	}
	for address, tokens := range *abs {
		for tokenAddr, value := range tokens {
			if value.Equal(decimal.Decimal{}) {
				delete(tokens, tokenAddr)
			}
		}
		if len(tokens) == 0 {
			delete(*abs, address)
		}
	}
}

func (abs *AssetBalances) ListPrices() (map[string]Token, error) {
	set := mapset.NewSet[string]()
	for address, tokens := range *abs {
		for tokenAddr := range tokens {
			set.Add(tokenAddr)
		}
		(*abs)[address] = tokens
	}
	tokens, err := UpdateTokensPrice(config.Conf.ETL.Chain, set.ToSlice())
	if err != nil {
		return nil, err
	}
	tokenMaps := map[string]Token{}
	for _, token := range tokens {
		tokenMaps[token.Address] = token
	}
	return tokenMaps, nil
}

func (e *Event) mapKeyExist(key string) bool {
	_, ok := (*e)[key]
	return ok
}

func convertAddress(origin string) string {
	end := len(origin)
	start := end - 40
	if len(origin) > 42 {
		return "0x" + origin[start:end]
	}
	return origin
}

func (as *Assets) analysisAssetTransfers(assetTransfers AssetTransfers, focuses []string) error {
	balances := AssetBalances{}
	balances.calcBalance(assetTransfers, focuses)
	tokensWithPrice, err := balances.ListPrices()
	if err != nil {
		return err
	}
	for address, tokens := range balances {
		asset := Asset{Address: address, TotalUSD: decimal.Decimal{}}
		assetTokens := []Token{}
		for tokenAddr, value := range tokens {
			token := tokensWithPrice[tokenAddr]
			token.Value = token.GetValueWithDecimals(value)
			assetTokens = append(assetTokens, token)
			if token.Price != nil {
				asset.TotalUSD.Add(token.Value.Mul(*token.Price))
			}
		}
		as.Items = append(as.Items, asset)
	}
	return nil
}

func (as *Assets) composeMsg() string {
	for _, asset := range as.Items {
		as.TotalUSD.Add(asset.TotalUSD)
	}
	chain := config.Conf.ETL.Chain
	scanURL := utils.GetScanURL(chain)
	items, _ := json.Marshal(as.Items)
	text := fmt.Sprintf("*Chain:* `%s`\n", strings.ToUpper(chain))
	text += fmt.Sprintf("*Block:* `%d`\n", as.BlockNumber)
	text += fmt.Sprintf("*DateTime:* `%s UTC`\n", time.Unix(as.BlockTimestamp, 0).Format(time.DateTime))
	text += fmt.Sprintf("*TXhash:* <%s|%s>\n", fmt.Sprintf("%s/tx/%s", scanURL, as.TxHash), as.TxHash)
	text += fmt.Sprintf("*Contract:* <%s|%s>\n", fmt.Sprintf("%s/address/%s", utils.GetScanURL(chain), as.ToAddress), as.ToAddress)
	text += fmt.Sprintf("*Assets:* %s\n\n", items)
	text += fmt.Sprintf("*Value USD:* %s\n\n", as.TotalUSD)
	return text
}

func (as *Assets) SendMessageToSlack() error {
	summary := fmt.Sprintf("⚠️Detected asset {asset_string} transfer on %s⚠️\n", config.Conf.ETL.Chain)
	attachment := slack.Attachment{
		Color:      "warning",
		AuthorName: "EXVul",
		Fallback:   summary,
		Text:       summary + as.composeMsg(),
		Footer:     fmt.Sprintf("skyeye-on-%s", config.Conf.ETL.Chain),
		Ts:         json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
	}
	msg := slack.WebhookMessage{
		Attachments: []slack.Attachment{attachment},
	}
	return slack.PostWebhook(config.Conf.ETL.SlackTransferWebHook, &msg)
}

func (as *Assets) alert() {
	if as.TotalUSD.Cmp(Threshold) >= 0 {
		stTime := time.Now()
		logrus.Infof("start to send asset alert msg to slack")
		if err := as.SendMessageToSlack(); err != nil {
			logrus.Errorf("send txhash %s's contract %s message to slack is err %v", as.TxHash, as.ToAddress, err)
		}
		logrus.Infof("send asset alert msg to slack is finished, cost %2.f", time.Since(stTime).Seconds())
	}
}

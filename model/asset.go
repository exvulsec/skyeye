package model

import (
	"context"
	"math/big"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"

	"github.com/exvulsec/skyeye/client"
)

type AssetTransfer struct {
	From    string
	To      string
	Value   decimal.Decimal
	Address string
}

const (
	TransferTopic     = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	WithdrawalTopic   = "0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65"
	DepositTopic      = "0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c"
	TransferABIName   = "Transfer"
	WithdrawalABIName = "Withdrawal"
	DepositABIName    = "Deposit"

	ABIs = `[
		{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"},
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

type Assets []Asset

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

	event := map[string]interface{}{}
	err = eventAbi.UnpackIntoMap(event, abiName, log.Data)
	if err != nil {
		return err
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

func (a *AssetTransfer) DecodeTransfer(event map[string]any, log types.Log) {
	if mapKeyExist(event, "from") && mapKeyExist(event, "to") {
		a.From = convertAddress(event["from"].(string))
		a.To = convertAddress(event["to"].(string))
	} else {
		a.From = convertAddress(log.Topics[1].String())
		a.To = convertAddress(log.Topics[2].String())
	}
	a.Value = decimal.NewFromBigInt(event["value"].(*big.Int), 0)
	a.Address = strings.ToLower(log.Address.String())
}

func (a *AssetTransfer) DecodeWithdrawal(event map[string]any, log types.Log) {
	if mapKeyExist(event, "src") {
		a.From = convertAddress(event["src"].(string))
	} else {
		a.From = convertAddress(log.Topics[1].String())
	}
	a.Value = decimal.NewFromBigInt(event["wad"].(*big.Int), 0)
	a.Address = strings.ToLower(log.Address.String())
}

func (a *AssetTransfer) DecodeDeposit(event map[string]any, log types.Log) {
	if mapKeyExist(event, "dst") {
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

func (abs *AssetBalances) calcBalance(transfers []AssetTransfer) {
	for _, transfer := range transfers {
		if !transfer.Value.Equal(decimal.Decimal{}) {
			abs.SetBalanceValue(transfer.From, transfer.Address, transfer.Value.Mul(decimal.NewFromInt(-1)))
			abs.SetBalanceValue(transfer.To, transfer.Address, transfer.Value)
		}
	}
}

func (abs *AssetBalances) filterEmptyBalance() {
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

func (abs *AssetBalances) ListPrices(chain string) (map[string]Token, error) {
	set := mapset.NewSet[string]()
	for address, tokens := range *abs {
		for tokenAddr := range tokens {
			set.Add(tokenAddr)
		}
		(*abs)[address] = tokens
	}
	tokens, err := UpdateTokensPrice(chain, set.ToSlice())
	if err != nil {
		return nil, err
	}
	tokenMaps := map[string]Token{}
	for _, token := range tokens {
		tokenMaps[token.Address] = token
	}
	return tokenMaps, nil
}

func (abs *AssetBalances) enrich(chain string) error {
	tokensWithPrice, err := abs.ListPrices(chain)
	if err != nil {
		return err
	}
	assets := Assets{}
	for address, tokens := range *abs {
		asset := Asset{Address: address, TotalUSD: decimal.Decimal{}}
		assetTokens := []Token{}
		for tokenAddr, value := range tokens {
			token := tokensWithPrice[tokenAddr]
			token.Value = token.GetValueWithDecimals(value)
			assetTokens = append(assetTokens, token)
			asset.TotalUSD.Add(token.Value.Mul(*token.Price))
		}
		assets = append(assets, asset)
	}
	return nil
}

func mapKeyExist(m map[string]interface{}, key string) bool {
	_, ok := m[key]
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

func GenerateLogs(chain, txHash string) []AssetTransfer {
	assetTransfers := []AssetTransfer{}
	r, err := client.MultiEvmClient()[chain].TransactionReceipt(context.TODO(), common.HexToHash(txHash))
	if err != nil {
		panic(err)
	}
	for _, l := range r.Logs {
		assetTransfer := AssetTransfer{}
		err := assetTransfer.Decode(*l)
		if err != nil {
			panic(err)
		}
		if assetTransfer.Address != "" {
			assetTransfers = append(assetTransfers, assetTransfer)
		}
	}
	return assetTransfers
}

package model

import (
	"context"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"
	"go-etl/client"
)

type CurrencyTransfer struct {
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

type CurrencyBalances map[string]map[string]Token

func (c *CurrencyTransfer) Decode(log types.Log) error {
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
	c.DecodeEvent(topic, event, log)

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

func (c *CurrencyTransfer) DecodeEvent(topic string, event map[string]any, log types.Log) {
	switch strings.ToLower(topic) {
	case TransferTopic:
		c.DecodeTransfer(event, log)
	case WithdrawalTopic:
		c.DecodeWithdrawal(event, log)
	case DepositTopic:
		c.DecodeDeposit(event, log)
	}
}

func (c *CurrencyTransfer) DecodeTransfer(event map[string]any, log types.Log) {
	if mapKeyExist(event, "from") && mapKeyExist(event, "to") {
		c.From = convertAddress(event["from"].(string))
		c.To = convertAddress(event["to"].(string))
	} else {
		c.From = convertAddress(log.Topics[1].String())
		c.To = convertAddress(log.Topics[2].String())
	}
	c.Value = decimal.NewFromBigInt(event["value"].(*big.Int), 0)
	c.Address = strings.ToLower(log.Address.String())
}

func (c *CurrencyTransfer) DecodeWithdrawal(event map[string]any, log types.Log) {
	if mapKeyExist(event, "src") {
		c.From = convertAddress(event["src"].(string))
	} else {
		c.From = convertAddress(log.Topics[1].String())
	}
	c.Value = decimal.NewFromBigInt(event["wad"].(*big.Int), 0)
	c.Address = strings.ToLower(log.Address.String())
}

func (c *CurrencyTransfer) DecodeDeposit(event map[string]any, log types.Log) {
	if mapKeyExist(event, "dst") {
		c.To = convertAddress(event["dst"].(string))
	} else {
		c.To = convertAddress(log.Topics[1].String())
	}
	c.Value = decimal.NewFromBigInt(event["wad"].(*big.Int), 0)
	c.Address = strings.ToLower(log.Address.String())
}

func (cbs *CurrencyBalances) SetBalanceValue(chain, address, token string, value decimal.Decimal) {
	if address == "" {
		return
	}
	_, ok := (*cbs)[address]
	if !ok {
		(*cbs)[address] = map[string]Token{}
	}
	_, ok = (*cbs)[address][token]
	if !ok {
		t := Token{
			Address: token,
		}
		existed := t.IsExisted(chain, token)
		if !existed {
			t.GetMetadataOnChain(chain, token)
		}
		(*cbs)[address][token] = Token{Value: value}
	} else {
		(*cbs)[address][token].Value.Add(value)
	}
}

func (cbs *CurrencyBalances) calcBalance(chain string, transfers []CurrencyTransfer) {
	for _, transfer := range transfers {
		if !transfer.Value.Equal(decimal.Decimal{}) {
			cbs.SetBalanceValue(chain, transfer.From, transfer.Address, transfer.Value.Mul(decimal.NewFromInt(-1)))
			cbs.SetBalanceValue(chain, transfer.To, transfer.Address, transfer.Value)
		}
	}
}

func (cbs *CurrencyBalances) filterEmptyBalance() {
	for address, tokens := range *cbs {
		for tokenAddr, token := range tokens {
			if token.Value.Equal(decimal.Decimal{}) {
				delete(tokens, tokenAddr)
			}
		}
		if len(tokens) == 0 {
			delete(*cbs, address)
		}
	}
}

func (cbs *CurrencyBalances) enrichDecimals() {
	for address, tokens := range *cbs {
		for tokenAddr, token := range tokens {
			token.Value = token.GetValueWithDecimals(token.Value)
			tokens[tokenAddr] = token
		}
		(*cbs)[address] = tokens
	}
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

func GenerateLogs(chain, txHash string) []CurrencyTransfer {
	currencyTransfers := []CurrencyTransfer{}
	r, err := client.MultiEvmClient()[chain].TransactionReceipt(context.TODO(), common.HexToHash(txHash))
	if err != nil {
		panic(err)
	}
	for _, l := range r.Logs {
		currencyTransfer := CurrencyTransfer{}
		err := currencyTransfer.Decode(*l)
		if err != nil {
			panic(err)
		}
		if currencyTransfer.Address != "" {
			currencyTransfers = append(currencyTransfers, currencyTransfer)
		}
	}
	return currencyTransfers
}

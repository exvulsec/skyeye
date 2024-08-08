package model

import (
	"fmt"
	"math/big"
	"strings"
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/utils"
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
}

type AssetTransfer struct {
	From    string
	To      string
	Value   decimal.Decimal
	Address string
}

type AssetTransfers []AssetTransfer

func (ats *AssetTransfers) Compose(logs []*types.Log, trace *TransactionTraceCall) {
	mutex := sync.RWMutex{}
	workers := make(chan int, 3)
	wg := sync.WaitGroup{}

	for _, l := range logs {
		workers <- 1
		wg.Add(1)
		go func() {
			defer func() {
				<-workers
				wg.Done()
			}()
			assetTransfer := AssetTransfer{}
			event, err := utils.Decode(*l)
			if err != nil {
				logrus.Error(err)
				return
			}
			if len(event) > 0 {
				assetTransfer.DecodeEvent(event, *l)
				mutex.Lock()
				if assetTransfer.Address != "" {
					*ats = append(*ats, assetTransfer)
				}
				mutex.Unlock()
			}
		}()
	}
	wg.Wait()
	if trace != nil {
		*ats = append(*ats, trace.ListTransferEvent()...)
	}
}

func (ats *AssetTransfers) ComposeFromTraceLogs(logs []TransactionTraceLog, txhash string) {
	mutex := sync.RWMutex{}
	workers := make(chan int, 3)
	wg := sync.WaitGroup{}

	for _, log := range logs {
		if len(log.Topics) > 0 {
			switch strings.ToLower(log.Topics[0].String()) {
			case utils.TransferTopic, utils.WithdrawalTopic, utils.DepositTopic:
				l := types.Log{
					Address: log.Address,
					Topics:  log.Topics,
					Data:    common.FromHex(log.Data),
					TxHash:  common.HexToHash(txhash),
				}
				workers <- 1
				wg.Add(1)
				go func() {
					defer func() {
						<-workers
						wg.Done()
					}()
					assetTransfer := AssetTransfer{}
					event, err := utils.Decode(l)
					if err != nil {
						logrus.Error(err)
						return
					}
					if len(event) > 0 {
						assetTransfer.DecodeEvent(event, l)
						mutex.Lock()
						if assetTransfer.Address != "" {
							*ats = append(*ats, assetTransfer)
						}
						mutex.Unlock()
					}
				}()
			default:
				continue
			}
		}
	}
	wg.Wait()
}

func (a *AssetTransfer) DecodeEvent(event map[string]any, log types.Log) {
	topic0 := strings.ToLower(log.Topics[0].String())
	switch strings.ToLower(topic0) {
	case utils.TransferTopic:
		a.DecodeTransfer(event, log)
	case utils.WithdrawalTopic:
		a.DecodeWithdrawal(event, log)
	case utils.DepositTopic:
		a.DecodeDeposit(event, log)
	}
}

func (a *AssetTransfer) DecodeTransfer(event Event, log types.Log) {
	a.From = convertAddress(event["from"].(common.Address).String())
	a.To = convertAddress(event["to"].(common.Address).String())
	a.Value = decimal.NewFromBigInt(event["value"].(*big.Int), 0)
	a.Address = strings.ToLower(log.Address.String())
}

func (a *AssetTransfer) DecodeWithdrawal(event Event, log types.Log) {
	a.From = convertAddress(event["src"].(common.Address).String())
	a.To = strings.ToLower(log.Address.String())
	a.Value = decimal.NewFromBigInt(event["wad"].(*big.Int), 0)
	a.Address = strings.ToLower(log.Address.String())
}

func (a *AssetTransfer) DecodeDeposit(event Event, log types.Log) {
	a.From = strings.ToLower(log.Address.String())
	a.To = convertAddress(event["dst"].(common.Address).String())
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
	workers := make(chan int, 3)
	wg := &sync.WaitGroup{}
	mutex := &sync.Mutex{}
	for _, transfer := range transfers {
		wg.Add(1)
		workers <- 1
		go func() {
			defer func() {
				<-workers
				wg.Done()
			}()
			if !transfer.Value.Equal(decimal.Decimal{}) {
				mutex.Lock()
				abs.SetBalanceValue(transfer.From, transfer.Address, transfer.Value.Mul(decimal.NewFromInt(-1)))
				abs.SetBalanceValue(transfer.To, transfer.Address, transfer.Value)
				mutex.Unlock()
			}
		}()
	}
	wg.Wait()
	abs.filterBalance()
}

func (abs *AssetBalances) filterBalance() {
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
		return strings.ToLower("0x" + origin[start:end])
	}
	return strings.ToLower(origin)
}

func (as *Assets) AnalysisAssetTransfers(assetTransfers AssetTransfers) error {
	balances := AssetBalances{}
	balances.calcBalance(assetTransfers)
	tokensWithPrice, err := balances.ListPrices()
	if err != nil {
		return err
	}
	workers := make(chan int, 3)
	wg := sync.WaitGroup{}
	mutex := sync.RWMutex{}
	for address, tokens := range balances {
		workers <- 1
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				<-workers
			}()
			asset := Asset{Address: address, TotalUSD: decimal.Decimal{}}
			assetTokens := []Token{}
			for tokenAddr, value := range tokens {
				token := tokensWithPrice[tokenAddr]
				token.Value = token.GetValueWithDecimals(value)
				token.ValueWithUnit = fmt.Sprintf("%s %s", token.Value, token.Symbol)
				token.ValueUSD = &decimal.Decimal{}
				if token.Price != nil {
					value := token.Value.Mul(*token.Price)
					token.ValueUSD = &value
					asset.TotalUSD = asset.TotalUSD.Add(value)
				}
				assetTokens = append(assetTokens, token)
			}
			asset.Tokens = assetTokens
			mutex.Lock()
			as.Items = append(as.Items, asset)
			mutex.Unlock()
		}()
	}
	wg.Wait()
	return nil
}

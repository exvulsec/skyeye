package model

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
	"github.com/exvulsec/skyeye/model/erc20"
	"github.com/exvulsec/skyeye/utils"
)

const (
	WETHAddress    = "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"
	WBNBAddress    = "0xbb4cdb9cbd36b01bd1cbaebf2de08d9173bc095c"
	ARBWETHAddress = "0x82af49447d8a07e3bd95bd0d56f35241523fbab1"
	WAVAXAddress   = "0xb31f66aa3c1e785363f0875a1b74e27b85fd66c7"
)

type Token struct {
	ID        *int64           `json:"-" gorm:"column:id"`
	Address   string           `json:"-" gorm:"column:address"`
	Name      string           `json:"-" gorm:"column:name"`
	Symbol    string           `json:"-" gorm:"column:symbol"`
	Decimals  int64            `json:"-" gorm:"column:decimals"`
	Value     decimal.Decimal  `json:"value" gorm:"-"`
	Price     *decimal.Decimal `json:"-" gorm:"column:price"`
	UpdatedAt time.Time        `json:"-" gorm:"column:updated_at"`
}

type Tokens []Token

func (t *Token) IsExisted(chain, address string) bool {
	err := datastore.DB().
		Table(utils.ComposeTableName(chain, datastore.TableTokens)).
		Where("address = ?", address).
		Find(t).Error
	if err != nil {
		logrus.Panic(err)
		return false
	}
	return t.ID != nil
}

func (t *Token) Create(chain string) error {
	return datastore.DB().
		Table(utils.ComposeTableName(chain, datastore.TableTokens)).
		Create(t).Error
}

func (t *Token) GetMetadataOnChain(chain, address string) error {
	token, err := erc20.NewErc20(common.HexToAddress(address), client.MultiEvmClient()[chain])
	if err != nil {
		return fmt.Errorf("failed to instantiate a token contract %s: %v", address, err)
	}
	name, err := token.Name(nil)
	if err != nil {
		return fmt.Errorf("get token %s name is err %v", address, err)
	}
	symbol, err := token.Symbol(nil)
	if err != nil {
		return fmt.Errorf("get token %s symbol is err %v", address, err)
	}

	decimals, err := token.Decimals(nil)
	if err != nil {
		return fmt.Errorf("get token %s decimals is err %v", address, err)
	}

	t.Address = address
	t.Name = name
	t.Symbol = symbol
	t.Decimals = int64(decimals)
	if err = t.Create(chain); err != nil {
		return fmt.Errorf("create token %s to db is err %v", address, err)
	}
	return nil
}

func (t *Token) GetSymbol() string {
	if t.Symbol == "" {
		return t.Address
	}
	return t.Symbol
}

func (t *Token) GetValueWithDecimals(value decimal.Decimal) decimal.Decimal {
	pow := decimal.NewFromInt(10).Pow(decimal.NewFromInt(t.Decimals))
	return value.DivRound(pow, 20)
}

func (t *Token) Update(chain string) error {
	return datastore.DB().
		Table(utils.ComposeTableName(chain, datastore.TableTokens)).
		Updates(t).Where("address = ?", t.Address).Error
}

func UpdateTokensPrice(chain string, tokenAddrs []string) (Tokens, error) {
	tokens := Tokens{}
	updateTokens := Tokens{}

	for _, addr := range tokenAddrs {
		token := Token{}
		if !token.IsExisted(chain, addr) {
			if err := token.GetMetadataOnChain(chain, addr); err != nil {
				return nil, err
			}
		}
		if token.Price == nil || time.Since(token.UpdatedAt) > time.Hour {
			updateTokens = append(updateTokens, token)
		} else {
			tokens = append(tokens, token)
		}
	}

	if len(updateTokens) > 0 {
		prices, err := updateTokens.GetCoinGeCkoPrices()
		if err != nil {
			return nil, err
		}
		for index, token := range updateTokens {
			if price, ok := prices[token.WrapperCurrencyAddress()]; ok {
				usdPrice := price["usd"]
				token.Price = &usdPrice
				updateTokens[index] = token
				if err := token.Update(chain); err != nil {
					logrus.Errorf("update token %s to db is err %v", token.Address, err)
					continue
				}
			}
		}
	}

	tokens = append(tokens, updateTokens...)

	return tokens, nil
}

func (ts *Tokens) GetCoinGeCkoPrices() (map[string]map[string]decimal.Decimal, error) {
	baseURL := `https://api.coingecko.com/api/v3/simple/token_price/%s?contract_addresses=%s&vs_currencies=usd`
	tokenAddrs := []string{}
	for _, token := range *ts {
		tokenAddrs = append(tokenAddrs, token.WrapperCurrencyAddress())
	}
	url := fmt.Sprintf(baseURL, utils.ConvertChainToCGCID(config.Conf.ETL.Chain), strings.Join(tokenAddrs, ","))
	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("x-cg-demo-api-key", config.Conf.ETL.CGCAPIKey)
	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("get cgc response is err: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	priceMaps := map[string]map[string]decimal.Decimal{}
	if err := json.Unmarshal(body, &priceMaps); err != nil {
		return nil, fmt.Errorf("unmarshal price map is err %v", err)
	}
	return priceMaps, nil
}

func (t *Token) WrapperCurrencyAddress() string {
	if t.Address == EVMPlatformCurrency {
		switch strings.ToLower(config.Conf.ETL.Chain) {
		case utils.ChainEthereum, utils.ChainEth:
			return WETHAddress
		case utils.ChainBSC:
			return WBNBAddress
		case utils.ChainArbitrum:
			return ARBWETHAddress
		case utils.ChainAvalanche:
			return WAVAXAddress
		}
	}
	return t.Address
}

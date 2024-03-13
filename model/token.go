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

	"go-etl/client"
	"go-etl/config"
	"go-etl/datastore"
	"go-etl/model/erc20"
	"go-etl/utils"
)

type Token struct {
	ID        *int64           `json:"id" gorm:"column:id"`
	Address   string           `json:"address" gorm:"column:address"`
	Name      string           `json:"name" gorm:"column:name"`
	Symbol    string           `json:"symbol" gorm:"column:symbol"`
	Decimals  int64            `json:"decimals" gorm:"column:decimals"`
	Value     decimal.Decimal  `json:"value" gorm:"column:-"`
	Price     *decimal.Decimal `json:"price" gorm:"column:price"`
	UpdatedAt time.Time        `json:"updated_at" gorm:"column:updated_at"`
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
		if token.Price != nil && time.Since(token.UpdatedAt) > time.Hour {
			updateTokens = append(updateTokens, token)
		} else {
			tokens = append(tokens, token)
		}
	}
	prices, err := updateTokens.GetCoinGeCkoPrices()
	if err != nil {
		return nil, err
	}
	for index, token := range updateTokens {
		if price, ok := prices[token.Address]; ok {
			p := price["usd"]
			token.Price = &p
			updateTokens[index] = token
			if err := token.Update(chain); err != nil {
				logrus.Errorf("update token %s to db is err %v", token.Address, err)
				continue
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
		tokenAddrs = append(tokenAddrs, token.Address)
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

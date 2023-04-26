package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/model"
	"go-etl/utils"
)

type AddressController struct{}

func (ac *AddressController) Routers(routers gin.IRouter) {
	api := routers.Group("/address")
	{
		api.GET("/:address/labels", ac.FindLabelByAddress)
		api.GET("/:address/associated", ac.AssociatedByAddress)
		api.GET("/:address/source_eth", ac.SourceETH)
		if config.Conf.HTTPServerConfig.SolidityCodePath != "" {
			api.GET("/:address/solidity", ac.ReadSolidityCode)
		}
	}
}

func (ac *AddressController) FindLabelByAddress(c *gin.Context) {
	addrLabel := model.AddressLabel{}
	chain := utils.GetChainFromQuery(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Param("address"))
	if err := addrLabel.GetLabels(chain, address); err != nil {
		c.JSON(
			http.StatusOK,
			model.Message{
				Code: http.StatusInternalServerError,
				Msg:  fmt.Sprintf("get address %s is err: %v", address, err),
			})
		return
	}
	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: addrLabel})
}

func (ac *AddressController) AssociatedByAddress(c *gin.Context) {
	chain := utils.GetChainFromQuery(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Param("address"))
	filterAddrs := strings.Split(c.Query("filter_addrs"), ",")
	txs := model.Transactions{}
	if len(filterAddrs) > 0 {
		if err := txs.FilterAssociatedAddrs(chain, address, filterAddrs); err != nil {
			c.JSON(
				http.StatusOK,
				model.Message{
					Code: http.StatusInternalServerError,
					Msg:  fmt.Sprintf("list the contract creation is err %v", err),
				})
			return
		}
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: txs})
}

func (ac *AddressController) SourceETH(c *gin.Context) {
	chain := utils.GetChainFromQuery(c.Query(utils.ChainKey))
	scanAPI := utils.GetScanAPI(chain)
	address := strings.ToLower(c.Param("address"))

	txResp := model.ScanTXResponse{}
	rand.Seed(time.Now().UnixNano())
	for {
		index := rand.Intn(len(config.Conf.HTTPServerConfig.EtherScanAPIKeys))
		ethScanAPIKEY := config.Conf.HTTPServerConfig.EtherScanAPIKeys[index]
		wg := sync.WaitGroup{}
		apis := []string{
			fmt.Sprintf(scanAPI, ethScanAPIKEY, address, utils.EtherScanTransactionAction),
			fmt.Sprintf(scanAPI, ethScanAPIKEY, address, utils.EtherScanTraceAction),
		}
		var (
			transaction model.ScanTransaction
			trace       model.ScanTransaction
		)

		for _, api := range apis {
			resp, err := client.HTTPClient().Get(api)
			if err != nil {
				c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get address %s's from scan api is err %v", address, err)})
				return
			}
			defer resp.Body.Close()
			base := model.ScanBaseResponse{}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("read body fro resp.Body is err %v", err)})
				return
			}
			if err = json.Unmarshal(body, &base); err != nil {
				c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("unmarshal json from body %s is err %v", string(body), err)})
				return
			}
			if base.Message == "NOTOK" {
				result := model.ScanStringResult{}
				if err = json.Unmarshal(body, &result); err != nil {
					c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("unmarshal json from body %s is err %v", string(body), err)})
					return
				}
				c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("get address %s error from etherscan %s", address, result.Result)})
				return
			}
			tx := model.ScanTransactionResponse{}
			if err = json.Unmarshal(body, &tx); err != nil {
				c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("unmarshal json from body %s is err %v", string(body), err)})
				return
			}
			if len(tx.Result) > 0 {
				if err = tx.Result[0].ConvertStringToInt(); err != nil {
					logrus.Errorf("convert string to int is err: %v", err)
					return
				}
				if strings.Contains(api, utils.EtherScanTraceAction) {
					trace = tx.Result[0]
				} else {
					transaction = tx.Result[0]
				}
			}
		}
		wg.Wait()
		address = transaction.FromAddress
		if transaction.Timestamp > trace.Timestamp && trace.Timestamp > 0 {
			address = trace.FromAddress
		}
		nonce, err := client.EvmClient().PendingNonceAt(context.Background(), common.HexToAddress(address))
		if err != nil {
			c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get nonce for address %s is err: %v", address, err)})
			return
		}
		txResp.Nonce = append(txResp.Nonce, nonce)
		if nonce >= config.Conf.HTTPServerConfig.AddressNonceThreshold || address == utils.EtherScanGenesisAddress {
			txResp.Address = address
			label := utils.EtherScanGenesisAddress
			if address != utils.EtherScanGenesisAddress {
				label = ac.getLabelFromMetaDock(chain, address)
			}
			txResp.Label = label
			break
		}
	}
	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: txResp})
}

func (ac *AddressController) ReadSolidityCode(c *gin.Context) {
	address := strings.ToLower(c.Param("address"))
	hexAddress := common.HexToAddress(address)
	byteCode, err := client.EvmClient().CodeAt(c, hexAddress, nil)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get byte code from ethereum is err: %v", err)})
		return
	}
	if hexutil.Encode(byteCode) == "0x" {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("the address %s has emtpy byte code", address)})
		return
	}

	fileName := fmt.Sprintf("%s/%s/%s.pan", config.Conf.HTTPServerConfig.SolidityCodePath, hexAddress, hexAddress)
	content, err := os.ReadFile(fileName)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("read file %s content is err: %v", fileName, err)})
		return
	}
	c.Data(http.StatusOK, "text/plain", content)
}

func (ac *AddressController) getLabelFromMetaDock(chain string, address string) string {
	headers := map[string]string{
		"authority":          "extension.blocksec.com",
		"accept":             "application/json",
		"blocksec-meta-dock": "v2.4.0",
		"user-agent":         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36",
		"content-type":       "application/json",
		"origin":             "chrome-extension://fkhgpeojcbhimodmppkbbliepkpcgcoo",
		"sec-fetch-site":     "none",
		"sec-fetch-mode":     "cors",
		"sec-fetch-dest":     "empty",
		"accept-encoding":    "gzip, deflate, br",
		"accept-language":    "zh,zh-CN;q=0.9",
	}

	metaDockChain := utils.ConvertChainToMetaDock(chain)
	body, err := json.Marshal(model.MetaDockLabelRequest{
		Chain: metaDockChain,
		Addresses: []string{
			address,
		},
	})
	if err != nil {
		logrus.Errorf("marhsall json from chain %s and address %s is err: %v", chain, address, err)
		return ""
	}

	req, err := http.NewRequest(http.MethodPost, "https://extension.blocksec.com/api/v1/address-label", bytes.NewBuffer(body))
	if err != nil {
		logrus.Errorf("new request for get meta dock labels is err: %v", err)
		return ""
	}
	for key, value := range headers {
		req.Header.Add(key, value)
	}

	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		logrus.Errorf("receive response from https://extension.blocksec.com/api/v1/address-label is err: %v", err)
		return ""
	}
	labels := model.MetaDockLabelsResponse{}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("read data from resp.Body is err: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if err = json.Unmarshal(data, &labels); err != nil {
		logrus.Errorf("unmarhsall data from resp.Body %s is err: %v", data, err)
		return ""
	}
	if len(labels) > 0 {
		return labels[0].Label
	}
	return ""
}

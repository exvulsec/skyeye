package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
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
				c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("get address %s from etherscan is err: %s, message is %s", address, result.Result, result.Message)})
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
		address = transaction.FromAddress
		if transaction.Timestamp > trace.Timestamp && trace.Timestamp > 0 {
			address = trace.FromAddress
		}
		var (
			nonce uint64
			err   error
		)

		if address != "" {
			nonce, err = client.EvmClient().PendingNonceAt(context.Background(), common.HexToAddress(address))
			if err != nil {
				c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get nonce for address %s is err: %v", address, err)})
				return
			}
			txResp.Nonce = append(txResp.Nonce, nonce)
		}
		if address == utils.EtherScanGenesisAddress || address == "" ||
			nonce >= config.Conf.HTTPServerConfig.AddressNonceThreshold {
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
	labels := model.MetaDockLabelsResponse{}
	if err := labels.GetLabels(chain, []string{address}); err != nil {
		logrus.Error(err)
		return ""
	}
	if len(labels) > 0 {
		return labels[0].Label
	}
	return ""
}

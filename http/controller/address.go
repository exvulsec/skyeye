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
		api.GET("/:address/labels", ac.GetAddressLabel)
		api.GET("/:address/associated", ac.AssociatedByAddress)
		api.GET("/:address/source_eth", ac.SourceETH)
		api.GET("/:address/dedaub", ac.GetDeDaub)
		var exposeSolidityCodeAPI = true
		for _, info := range config.Conf.ScanInfos {
			if info.SolidityCodePath == "" {
				exposeSolidityCodeAPI = false
				break
			}
		}
		if exposeSolidityCodeAPI {
			api.GET("/:address/solidity", ac.ReadSolidityCode)
		}
	}
}

func (ac *AddressController) GetAddressLabel(c *gin.Context) {
	chain := utils.GetChainFromQuery(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Param("address"))
	label := model.AddressLabel{}
	if err := label.GetLabel(chain, address); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: label})
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
	scanAPI := fmt.Sprintf("%s%s", utils.GetScanAPI(chain), utils.APIQuery)
	address := strings.ToLower(c.Param("address"))

	txResp := model.ScanTXResponse{}
	rand.Seed(time.Now().UnixNano())
	for {
		scanInfo := config.Conf.ScanInfos[chain]
		index := rand.Intn(len(scanInfo.APIKeys))
		scanAPIKEY := scanInfo.APIKeys[index]
		apis := []string{
			fmt.Sprintf(scanAPI, scanAPIKEY, address, utils.ScanTransactionAction),
			fmt.Sprintf(scanAPI, scanAPIKEY, address, utils.ScanTraceAction),
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
				if strings.Contains(api, utils.ScanTraceAction) {
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
			nonce, err = client.MultiEvmClient()[chain].PendingNonceAt(context.Background(), common.HexToAddress(address))
			if err != nil {
				c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get nonce for address %s is err: %v", address, err)})
				return
			}
			txResp.Nonce = append(txResp.Nonce, nonce)
		}
		if address == utils.ScanGenesisAddress || address == "" ||
			nonce >= scanInfo.AddressNonceThreshold || len(txResp.Nonce) == 5 {
			txResp.Address = address
			label := utils.ScanGenesisAddress
			if address != utils.ScanGenesisAddress && address != "" {
				addrLabel := model.AddressLabel{}
				if err = addrLabel.GetLabel(chain, address); err != nil {
					c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get address %s label is err: %v", address, err)})
					return
				}
				label = addrLabel.Label
			}
			txResp.Label = label
			break
		}
	}
	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: txResp})
}

func (ac *AddressController) ReadSolidityCode(c *gin.Context) {
	chain := utils.GetChainFromQuery(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Param("address"))
	hexAddress := common.HexToAddress(address)
	multiClient := client.MultiEvmClient()
	byteCode, err := multiClient[chain].CodeAt(c, hexAddress, nil)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get byte code from ethereum is err: %v", err)})
		return
	}
	if hexutil.Encode(byteCode) == "0x" {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("the address %s is not a contract address", address)})
		return
	}

	scanInfo := config.Conf.ScanInfos[chain]
	fileName := fmt.Sprintf("%s/%s/%s.pan", scanInfo.SolidityCodePath, hexAddress, hexAddress)
	content, err := os.ReadFile(fileName)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("read file %s content is err: %v", fileName, err)})
		return
	}
	c.Data(http.StatusOK, "text/plain", content)
}

func (ac *AddressController) GetDeDaub(c *gin.Context) {
	chain := utils.GetChainFromQuery(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Param("address"))
	dedaub := model.DeDaub{
		Chain:   chain,
		Address: address,
	}
	if err := dedaub.Get(); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get chain %s address %s's dedaub is err: %v", chain, address, err)})
		return
	}

	if dedaub.MD5 == "" || len(dedaub.MD5) != 32 {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("chain %s address %s's dedaub is not post or invalid", chain, address)})
		return
	}
	data, err := ac.getSource(chain, address, dedaub.MD5)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
	}
	c.Data(http.StatusOK, "text/plain", data)
}

func (ac *AddressController) getSource(chain, address, md5 string) ([]byte, error) {
	var (
		data []byte
		err  error
	)
	fileName := fmt.Sprintf("%s/%s/%s-%s.source", config.Conf.HTTPServer.DeDaubCodePath, chain, address, md5)
	_, err = os.Stat(fileName)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("check file %s status is err %v", fileName, err)
		}
		dr := model.DeDaubResponse{}
		if err = dr.GetSource(md5); err != nil {
			return nil, fmt.Errorf("get chain %s address %s's dedaub source is err %v", chain, address, err)
		}
		data = []byte(dr.Source)
		if err = os.WriteFile(fileName, []byte(dr.Source), 0644); err != nil {
			return nil, fmt.Errorf("write source to file %s is err %v", fileName, err)
		}
		return data, nil
	}
	data, err = os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("read source file %s is err %v", fileName, err)
	}
	return data, nil
}

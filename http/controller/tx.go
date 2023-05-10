package controller

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"

	"go-etl/client"
	"go-etl/config"
	"go-etl/model"
	"go-etl/policy"
	"go-etl/utils"
)

type TXController struct{}

func (tc *TXController) Routers(routers gin.IRouter) {
	api := routers.Group("/tx")
	{
		api.GET("/reviewed", tc.Reviewed)
	}
}

func (tc *TXController) Reviewed(c *gin.Context) {
	scanURL := c.Query("scanurl")
	var (
		chain  string
		txhash string
	)
	if scanURL == "" {
		chain = utils.GetChainFromQuery(c.Query(utils.ChainKey))
		txhash = strings.ToLower(c.Query("txhash"))
	} else {
		chain = utils.GetChainFromScanURL(scanURL)
		txhash = utils.GetTXHashFromScanURL(scanURL)
		if txhash == "" {
			c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("invalid txhash in url %s", scanURL)})
			return
		}
	}

	var (
		evmClient *ethclient.Client
		ok        bool
	)
	if evmClient, ok = client.MultiEvmClient()[chain]; !ok {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("not foudn evm client by given chain: %s", chain)})
		return
	}
	tx, _, err := evmClient.TransactionByHash(c, common.HexToHash(txhash))
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get transcation %s from rpc node is err %v ", txhash, err)})
		return
	}
	if tx.To() != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("get transcation %s from rpc node is err %v ", txhash, err)})
		return
	}

	receipt, err := evmClient.TransactionReceipt(c, common.HexToHash(txhash))
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get transcation %s's receipt from rpc node is err %v ", txhash, err)})
		return
	}

	code, err := evmClient.CodeAt(context.Background(), receipt.ContractAddress, nil)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get contract %s's bytecode is err %v ", receipt.ContractAddress.String(), err)})
		return
	}
	policyCode, err := policy.FilterContractByPolicy(chain, receipt.ContractAddress.String(), tx.Nonce(), config.Conf.HTTPServer.NonceThreshold, 0, code)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("filter txhash's contract %s by policy is err %v", receipt.ContractAddress.String(), err)})
		return
	}

	if policyCode == policy.NoAnyDenied {
		values, err := policy.SendItemToMessageQueue(chain, txhash,
			receipt.ContractAddress.String(), "http://localhost:8088", code, false)
		if err != nil {
			c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("send to txhash %s's contract %s message queue is err %v", txhash, receipt.ContractAddress.String(), err)})
			return
		}
		c.JSON(http.StatusOK, model.Message{Code: policyCode, Msg: fmt.Sprintf("contract address %s is %s", receipt.ContractAddress.String(), policy.DeniedMap[policyCode]), Data: values})
		return
	}

	c.JSON(http.StatusOK, model.Message{Code: policyCode, Msg: fmt.Sprintf("contract address %s is %s", receipt.ContractAddress.String(), policy.DeniedMap[policyCode])})
}

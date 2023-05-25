package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"

	"go-etl/client"
	"go-etl/config"
	"go-etl/model"
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
	weightStrings := c.Query("weights")
	var weights = make([]string, 6)
	if weightStrings != "" {
		weights = strings.Split(weightStrings, ",")
	} else {
		for i := range weights {
			weights[i] = "1"
		}
	}

	var (
		ethClient *ethclient.Client
		ok        bool
	)
	if ethClient, ok = client.MultiEvmClient()[chain]; !ok {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("not foudn evm client by given chain: %s", chain)})
		return
	}
	tx, _, err := ethClient.TransactionByHash(c, common.HexToHash(txhash))
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get transcation %s from rpc node is err %v ", txhash, err)})
		return
	}

	receipt, err := ethClient.TransactionReceipt(c, common.HexToHash(txhash))
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get transcation %s's receipt from rpc node is err %v ", txhash, err)})
		return
	}

	if tx.To() != nil || receipt.ContractAddress.String() == utils.ZeroAddress {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("transaction %s is not a contract creation", txhash)})
		return
	}

	code, err := ethClient.CodeAt(context.Background(), receipt.ContractAddress, nil)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get contract %s's bytecode is err %v ", receipt.ContractAddress.String(), err)})
		return
	}
	block, err := ethClient.BlockByHash(c, receipt.BlockHash)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get contract %s's block number is err %v ", receipt.ContractAddress.String(), err)})
		return
	}

	nt := model.NastiffTransaction{
		Chain:           chain,
		BlockNumber:     block.Number().Int64(),
		BlockTimestamp:  int64(block.Time()),
		TxHash:          tx.Hash().String(),
		TxPos:           int64(receipt.TransactionIndex),
		ContractAddress: receipt.ContractAddress.String(),
		Nonce:           tx.Nonce(),
		ByteCode:        code,
	}
	policies := []model.FilterPolicy{
		&model.NonceFilter{ThresholdNonce: config.Conf.HTTPServer.NonceThreshold},
		&model.ByteCodeFilter{},
		&model.ContractTypeFilter{},
		&model.OpenSourceFilter{},
		&model.Push4ArgsFilter{},
		&model.Push20ArgsFilter{},
	}

	policyResults := []string{}
	score := 0
	totalScore := 0
	for index, p := range policies {
		result := "1"
		weight, err := strconv.Atoi(weights[index])
		if p.ApplyFilter(&nt) {
			result = "0"
		} else {
			if err != nil {
				c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("convert weight %s to int is err: %v", weights[index], err)})
				return
			}
			score += weight
		}
		policyResults = append(policyResults, result)
		totalScore += weight
	}
	nt.Policies = strings.Join(policyResults, ",")
	nt.Score = score * 100 / totalScore
	if err = nt.ComposeNastiffValues(false, "http://localhost:8088"); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get contract %s's nastiff values is err %v ", receipt.ContractAddress.String(), err)})
		return
	}

	nt.NastiffValues["policies"] = nt.Policies
	c.JSON(http.StatusOK, model.Message{
		Code: http.StatusOK,
		Msg:  "",
		Data: nt.NastiffValues,
	})
}

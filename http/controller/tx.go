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
	var weights = make([]string, 7)
	if weightStrings != "" {
		weights = strings.Split(weightStrings, ",")
	} else {
		for i := range weights {
			weights[i] = "1"
		}
	}

	searchFund, _ := strconv.ParseBool(c.Query("search_fund"))

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
	policies := []model.PolicyCalc{
		&model.NoncePolicyCalc{ThresholdNonce: config.Conf.HTTPServer.NonceThreshold},
		&model.ByteCodePolicyCalc{},
		&model.ContractTypePolicyCalc{},
		&model.OpenSourcePolicyCalc{Interval: config.Conf.ETL.ScanInterval},
		&model.Push4PolicyCalc{FlashLoanFuncNames: model.LoadFlashLoanFuncNames()},
		&model.Push20PolicyCalc{},
		&model.FundPolicyCalc{IsNastiff: searchFund, OpenAPIServer: "http://localhost:8088"},
	}
	splitScores := []string{}
	totalScore := 0
	for index, p := range policies {
		weight, err := strconv.Atoi(weights[index])
		if err != nil {
			c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("convert weight %s to int is err: %v", weights[index], err)})
		}
		score := p.Calc(&nt)
		splitScores = append(splitScores, fmt.Sprintf("%d", score))
		totalScore += score * weight
	}
	nt.Score = totalScore
	nt.SplitScores = splitScores
	if err = nt.ComposeNastiffValues(); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get contract %s's nastiff values is err %v ", receipt.ContractAddress.String(), err)})
		return
	}
	c.JSON(http.StatusOK, model.Message{
		Code: http.StatusOK,
		Msg:  "",
		Data: nt.NastiffValues,
	})
}

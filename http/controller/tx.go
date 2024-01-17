package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"go-etl/client"
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

func (tc *TXController) ReviewedCompose(st *model.SkyEyeTransaction, searchFund bool, weights []string) error {
	policies := []model.PolicyCalc{
		&model.NoncePolicyCalc{},
		&model.ByteCodePolicyCalc{},
		&model.ContractTypePolicyCalc{},
		&model.Push4PolicyCalc{FlashLoanFuncNames: model.LoadFlashLoanFuncNames()},
		&model.Push20PolicyCalc{},
		&model.FundPolicyCalc{IsNastiff: searchFund},
	}
	splitScores := []string{}
	totalScore := 0
	for index, p := range policies {
		weight, err := strconv.Atoi(weights[index])
		if err != nil {
			return fmt.Errorf("convert weight %s to int is err: %v", weights[index], err)
		}
		score := p.Calc(st)
		splitScores = append(splitScores, fmt.Sprintf("%d", score))
		totalScore += score * weight
	}
	st.Score = totalScore
	st.SplitScores = strings.Join(splitScores, ",")
	return nil
}

func (tc *TXController) Reviewed(c *gin.Context) {
	scanURL := c.Query("scanurl")
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
	contractAddress := c.Query("contract_address")
	txhash := strings.ToLower(c.Query("txhash"))
	if txhash == "" && contractAddress == "" && scanURL != "" {
		chain = utils.GetChainFromScanURL(scanURL)
		txhash = utils.GetTXHashFromScanURL(scanURL)
		if txhash == "" {
			c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("invalid txhash in url %s", scanURL)})
			return
		}
	}
	if txhash == "" && contractAddress == "" {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: "required txhash or contract_address as parameter"})
		return
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

	if contractAddress != "" {
		code, err := ethClient.CodeAt(context.Background(), common.HexToAddress(contractAddress), nil)
		if err != nil {
			c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get contract %s's bytecode is err %v ", contractAddress, err)})
			return
		}

		st := model.SkyEyeTransaction{
			Chain:           chain,
			FromAddress:     contractAddress,
			ContractAddress: contractAddress,
			ByteCode:        code,
		}
		if err = tc.ReviewedCompose(&st, searchFund, weights); err != nil {
			c.JSON(http.StatusOK, model.Message{
				Code: http.StatusBadRequest,
				Msg:  "",
				Data: st.ComposeSkyEyeTXValues(),
			})
			return
		}
		c.JSON(http.StatusOK, model.Message{
			Code: http.StatusOK,
			Msg:  "",
			Data: st.ComposeSkyEyeTXValues(),
		})
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
	contractAddress = receipt.ContractAddress.String()

	code, err := ethClient.CodeAt(context.Background(), common.HexToAddress(contractAddress), nil)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get contract %s's bytecode is err %v ", receipt.ContractAddress.String(), err)})
		return
	}
	block, err := ethClient.BlockByHash(c, receipt.BlockHash)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get contract %s's block number is err %v ", receipt.ContractAddress.String(), err)})
		return
	}

	fromAddr, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		logrus.Fatalf("get from address is err: %v", err)
	}

	st := model.SkyEyeTransaction{
		Chain:           chain,
		BlockNumber:     block.Number().Int64(),
		BlockTimestamp:  int64(block.Time()),
		TxHash:          tx.Hash().String(),
		FromAddress:     fromAddr.String(),
		TxPos:           int64(receipt.TransactionIndex),
		ContractAddress: receipt.ContractAddress.String(),
		Nonce:           tx.Nonce(),
		ByteCode:        code,
	}
	if err = tc.ReviewedCompose(&st, searchFund, weights); err != nil {
		c.JSON(http.StatusOK, model.Message{
			Code: http.StatusBadRequest,
			Msg:  "",
			Data: st.ComposeSkyEyeTXValues(),
		})
		return
	}

	c.JSON(http.StatusOK, model.Message{
		Code: http.StatusOK,
		Msg:  "",
		Data: st.ComposeSkyEyeTXValues(),
	})
}

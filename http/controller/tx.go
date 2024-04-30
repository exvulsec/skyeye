package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
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

	searchFund, _ := strconv.ParseBool(c.Query("search_fund"))
	var (
		ethClient *ethclient.Client
		ok        bool
	)
	if ethClient, ok = client.MultiEvmClient()[chain]; !ok {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("not foudn evm client by given chain: %s", chain)})
		return
	}

	ethTX, _, err := ethClient.TransactionByHash(c, common.HexToHash(txhash))
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get transcation %s from rpc node is err %v ", txhash, err)})
		return
	}

	tx := model.Transaction{}
	tx.ConvertFromBlock(ethTX)
	tx.EnrichReceipt(chain)
	tx.GetTrace(chain)

	block, err := ethClient.BlockByHash(c, tx.Receipt.BlockHash)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get contract %s's block number is err %v ", tx.Receipt.ContractAddress.String(), err)})
		return
	}

	policies := []model.PolicyCalc{
		&model.MultiContractCalc{},
		&model.FundPolicyCalc{Chain: chain, NeedFund: searchFund},
		&model.NoncePolicyCalc{},
	}
	results := []model.SkyEyeTransaction{}

	skyTx := model.SkyEyeTransaction{}
	skyTx.ConvertFromTransaction(tx)
	for _, p := range policies {
		if p.Filter(&skyTx) {
			return
		}
		score := p.Calc(&skyTx)
		skyTx.Scores = append(skyTx.Scores, fmt.Sprintf("%s: %d", p.Name(), score))
		skyTx.Score += score
	}

	skyTx.BlockNumber = block.Number().Int64()
	skyTx.BlockTimestamp = int64(block.Time())

	for _, contract := range skyTx.MultiContracts {
		contractTX := model.SkyEyeTransaction{
			Chain:               skyTx.Chain,
			BlockTimestamp:      skyTx.BlockTimestamp,
			BlockNumber:         skyTx.BlockNumber,
			TxHash:              skyTx.TxHash,
			TxPos:               skyTx.TxPos,
			FromAddress:         skyTx.FromAddress,
			ContractAddress:     contract,
			Nonce:               skyTx.Nonce,
			Score:               skyTx.Score,
			Scores:              skyTx.Scores,
			Fund:                skyTx.Fund,
			MultiContractString: skyTx.MultiContractString,
		}
		contractTX.Analysis(chain)
		contractTX.ComposeSkyEyeTXValues()
		results = append(results, contractTX)
	}

	c.JSON(http.StatusOK, model.Message{
		Code: http.StatusOK,
		Msg:  "",
		Data: results,
	})
}

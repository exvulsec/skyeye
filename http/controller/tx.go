package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

/*
#cgo LDFLAGS: -L../../lib -lsimulation
#include "../../lib/simulation.h"
*/
import "C"

type TXController struct{}

func (tc *TXController) Routers(routers gin.IRouter) {
	api := routers.Group("/tx")
	{
		api.GET("/reviewed", tc.Reviewed)
		api.GET("/:tx_hash/graph", tc.TransactionFundFlowGraph)
		api.GET("/:tx_hash/contract/:contract/simulation", tc.Simulation)
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
	tx.ConvertFromBlock(ethTX, 0)
	tx.EnrichReceipt(chain)
	tx.GetTrace(chain)
	contracts, skip := tx.Trace.ListContracts()
	if skip {
		return
	}

	block, err := ethClient.BlockByHash(c, tx.Receipt.BlockHash)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get contract %s's block number is err %v ", tx.Receipt.ContractAddress.String(), err)})
		return
	}

	policies := []model.PolicyCalc{
		&model.FundPolicyCalc{Chain: chain, NeedFund: searchFund},
		&model.NoncePolicyCalc{},
	}
	results := []model.SkyEyeTransaction{}

	skyTx := model.SkyEyeTransaction{MultiContracts: contracts}
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
			Chain:               chain,
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

func (tc *TXController) TransactionFundFlowGraph(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
	txhash := strings.ToLower(c.Param("tx_hash"))
	transaction, _, err := client.MultiEvmClient()[chain].TransactionByHash(context.Background(), common.HexToHash(txhash))
	if err != nil {
		c.JSON(http.StatusOK, model.Message{
			Code: http.StatusInternalServerError,
			Msg:  fmt.Sprintf("get transaction is err %v", err),
		})
		return
	}

	tx := model.Transaction{}
	tx.ConvertFromBlock(transaction, 0)
	tx.TxHash = txhash
	tx.BlockTimestamp = transaction.Time().Unix()
	graph, err := tx.GenerateFundFlowGraph(chain)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{
			Code: http.StatusBadRequest,
			Msg:  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, model.Message{
		Code: 0,
		Data: graph,
	})
}

func ExecuteSimulation(chain, txhash, contract string) (string, error) {
	cChain := C.CString(chain)
	defer C.free(unsafe.Pointer(cChain))

	cTxhash := C.CString(txhash)
	defer C.free(unsafe.Pointer(cTxhash))

	cContract := C.CString(contract)
	defer C.free(unsafe.Pointer(cContract))

	cValue := C.CString("0")
	defer C.free(unsafe.Pointer(cValue))

	opt := C.OptFFI{
		txhash:     cTxhash,
		contract:   cContract,
		followcall: 0,
		chain:      cChain,
		calldata:   nil,
		instrace:   0,
		value:      cValue,
		is_json:    1,
	}

	var success C.bool
	result := C.get_simulation_json(&opt, &success)
	defer C.free_simulation_json(result)

	goString := C.GoString(result)

	if !bool(success) {
		return "", fmt.Errorf("got simulation is err: %v", goString)
	}

	return goString, nil
}

func (tc *TXController) Simulation(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
	if chain == "" {
		c.JSON(http.StatusOK, model.Message{
			Code: http.StatusBadRequest,
			Msg:  "chain is empty",
		})
		return
	}

	txhash := strings.ToLower(c.Param("tx_hash"))
	contract := strings.ToLower(c.Param("contract"))
	result, err := ExecuteSimulation(chain, txhash, contract)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{
			Code: http.StatusInternalServerError,
			Msg:  fmt.Sprintf("simulation is err: %v", err),
		})
		return
	}
	c.JSON(http.StatusOK, model.Message{
		Code: 0,
		Data: result,
	})
}

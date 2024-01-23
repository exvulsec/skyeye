package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"

	"go-etl/client"
	"go-etl/model"
	"go-etl/utils"
)

type SkyEyeController struct{}

func (sc *SkyEyeController) Routers(routers gin.IRouter) {
	api := routers.Group("/skyeye")
	{
		api.POST("/latest", sc.GetLatestBlockNumber)
		api.POST("/decode", sc.DecodeByteCode)
	}
}

func (sc *SkyEyeController) GetLatestBlockNumber(c *gin.Context) {
	chain := utils.GetSupportChain(c.PostForm("text"))

	latestBlock, err := client.MultiEvmClient()[chain].BlockNumber(c)
	if err != nil {
		c.String(http.StatusOK, fmt.Sprintf("get latest block from rpc node for chain %s is err %v", chain, err))
		return
	}
	stx := model.SkyEyeTransaction{}
	if err = stx.GetLatestRecord(chain); err != nil {
		c.String(http.StatusOK, fmt.Sprintf("get latest record from skyeye table for chain %s is err %v", chain, err))
		return
	}
	text := fmt.Sprintf("%s block number: `%d`\n", chain, latestBlock)
	text += fmt.Sprintf("SkyEye processed block number: `%d`\n", stx.BlockNumber)

	c.String(http.StatusOK, text)
}

func (sc *SkyEyeController) DecodeByteCode(c *gin.Context) {
	var request = struct {
		ByteCode string `json:"byte_code"`
	}{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusBadRequest, Msg: fmt.Sprintf("unmarshal the input bytecode to json is err %v", err)})
		return
	}
	skyTx := model.SkyEyeTransaction{}
	skyTx.ByteCode = append([]byte("0x"), common.FromHex(request.ByteCode)...)
	sc.GetScoreFromByteCode(&skyTx)
	values := skyTx.ComposeSkyEyeTXValuesFromByteCode()
	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: values})
}

func (sc *SkyEyeController) GetScoreFromByteCode(skyTx *model.SkyEyeTransaction) {
	policies := []model.PolicyCalc{
		&model.ByteCodePolicyCalc{},
		&model.ContractTypePolicyCalc{},
		&model.Push4PolicyCalc{FlashLoanFuncNames: model.LoadFlashLoanFuncNames()},
		&model.Push20PolicyCalc{},
	}
	splitScores := []string{}
	totalScore := 0
	for _, p := range policies {
		score := p.Calc(skyTx)
		splitScores = append(splitScores, fmt.Sprintf("%s: %d", p.Name(), score))
		totalScore += score
	}
	skyTx.Score = totalScore
	skyTx.SplitScores = strings.Join(splitScores, " ")
}

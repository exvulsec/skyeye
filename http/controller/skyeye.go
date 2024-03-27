package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/model"
	"github.com/exvulsec/skyeye/utils"
)

type SkyEyeController struct{}

func (sc *SkyEyeController) Routers(routers gin.IRouter) {
	api := routers.Group("/skyeye")
	{
		api.POST("/latest", sc.GetLatestBlockNumber)
	}
}

func (sc *SkyEyeController) GetLatestBlockNumber(c *gin.Context) {
	chain := utils.GetSupportChain(c.PostForm("text"))

	latestBlock, err := client.MultiEvmClient()[chain].BlockNumber(c)
	if err != nil {
		c.String(http.StatusOK, fmt.Sprintf("get latest block from rpc node for chain %s is err %v", chain, err))
		return
	}
	stx := model.Transaction{}
	if err = stx.GetLatestRecord(chain); err != nil {
		c.String(http.StatusOK, fmt.Sprintf("get latest record from skyeye table for chain %s is err %v", chain, err))
		return
	}
	text := fmt.Sprintf("%s block number: `%d`\n", chain, latestBlock)
	text += fmt.Sprintf("SkyEye processed block number: `%d`\n", stx.BlockNumber)

	c.String(http.StatusOK, text)
}

func (sc *SkyEyeController) GetScoreFromByteCode(skyTx *model.SkyEyeTransaction) {
	policies := []model.PolicyCalc{
		&model.ByteCodePolicyCalc{},
		&model.ContractTypePolicyCalc{},
		&model.Push4PolicyCalc{FlashLoanFuncNames: model.FuncNameList},
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

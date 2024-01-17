package controller

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gin-gonic/gin"

	"go-etl/client"
	"go-etl/config"
	"go-etl/model"
	"go-etl/utils"
)

type AddressController struct {
	fpc model.FundPolicyCalc
}

func (ac *AddressController) Routers(routers gin.IRouter) {
	api := routers.Group("/address")
	{
		api.GET("/:address/labels", ac.GetAddressLabel)
		api.GET("/:address/associated", ac.AssociatedByAddress)
		api.GET("/:address/fund", ac.GetFund)
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
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Param("address"))
	label := model.AddressLabel{}
	if err := label.GetLabel(chain, address); err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: label})
}

func (ac *AddressController) AssociatedByAddress(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
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

func (ac *AddressController) GetFund(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))

	address := strings.ToLower(c.Param("address"))
	txResp, err := ac.fpc.SearchFund(chain, address)
	if err != nil {
		c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: err.Error()})
	}

	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: txResp})
}

func (ac *AddressController) ReadSolidityCode(c *gin.Context) {
	chain := utils.GetSupportChain(c.Query(utils.ChainKey))
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

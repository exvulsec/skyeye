package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
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
		api.GET("/:address/labels", ac.FindLabelByAddress)
		api.GET("/:address/associated", ac.AssociatedByAddress)
		api.GET("/:address/source_eth", ac.SourceETH)
		if config.Conf.HTTPServerConfig.ReadSolidityCode {
			api.GET("/:address/solidity_code", ac.ReadSolidityCode)
		}
	}
}

func (ac *AddressController) FindLabelByAddress(c *gin.Context) {
	addrLabel := model.AddressLabel{}
	chain := utils.GetChainFromQuery(c.Query(utils.ChainKey))
	address := strings.ToLower(c.Param("address"))
	if err := addrLabel.GetLabels(chain, address); err != nil {
		c.JSON(
			http.StatusOK,
			model.Message{
				Code: http.StatusInternalServerError,
				Msg:  fmt.Sprintf("get address %s is err: %v", address, err),
			})
		return
	}
	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: addrLabel})
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
	scanAPI := utils.GetScanAPI(chain)
	address := strings.ToLower(c.Param("address"))

	txResp := model.ScanTXResponse{}
	rand.Seed(time.Now().UnixNano())
	for {
		index := rand.Intn(len(config.Conf.HTTPServerConfig.EtherScanAPIKeys))
		ethScanAPIKEY := config.Conf.HTTPServerConfig.EtherScanAPIKeys[index]
		wg := sync.WaitGroup{}
		apis := []string{
			fmt.Sprintf(scanAPI, ethScanAPIKEY, address, utils.EtherScanTransactionAction),
			fmt.Sprintf(scanAPI, ethScanAPIKEY, address, utils.EtherScanTraceAction),
		}
		var (
			transaction model.ScanTransaction
			trace       model.ScanTransaction
		)

		for _, api := range apis {
			resp, err := client.HTTPClient().Get(api)
			if err != nil {
				logrus.Errorf("get address %s's from scan api is err %v", address, err)
				return
			}
			defer resp.Body.Close()
			tx := model.ScanTransactionResponse{}
			if err = json.NewDecoder(resp.Body).Decode(&tx); err != nil {
				logrus.Errorf("json unmarshall from %s is err %v,", resp.Body, err)
				return
			}
			if tx.Status == "1" {
				if len(tx.Result) > 0 {
					if err = tx.Result[0].ConvertStringToInt(); err != nil {
						logrus.Errorf("convert string to int is err: %v", err)
						return
					}
					if strings.Contains(api, utils.EtherScanTraceAction) {
						trace = tx.Result[0]
					} else {
						transaction = tx.Result[0]
					}
				}
			}
		}
		wg.Wait()
		address = transaction.FromAddress
		if transaction.Timestamp > trace.Timestamp && trace.Timestamp > 0 {
			address = trace.FromAddress
		}
		nonce, err := client.EvmClient().PendingNonceAt(context.Background(), common.HexToAddress(address))
		if err != nil {
			c.JSON(http.StatusOK, model.Message{Code: http.StatusInternalServerError, Msg: fmt.Sprintf("get nonce for address %s is err: %v", address, err)})
			return
		}
		txResp.Nonce = append(txResp.Nonce, nonce)
		if nonce >= config.Conf.HTTPServerConfig.AddressNonceThreshold {
			txResp.Address = address
			label := model.AddressLabel{}
			if err = label.GetLabels(chain, address); err != nil {
				logrus.Errorf("get address %s's label is err: %v", address, err)
				return
			}
			txResp.Label = label.Name
			break
		}
	}
	c.JSON(http.StatusOK, model.Message{Code: http.StatusOK, Data: txResp})
}

func (ac *AddressController) ReadSolidityCode(c *gin.Context) {
	return
}

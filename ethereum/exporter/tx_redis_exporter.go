package exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/datastore"
	"go-etl/model"
	"go-etl/utils"
)

var (
	TransactionAssociatedAddrs       = "%s:txs_associated:addrs"
	TransactionContractAddressStream = "%s:contract_address:stream"
)

type TransactionRedisExporter struct {
	Chain         string
	Nonce         uint64
	OpenAPIServer string
}

func NewTransactionExporters(chain, table, openAPIServer string, nonce uint64) []Exporter {
	exporters := []Exporter{}
	if config.Conf.Postgresql.Host != "" {
		exporters = append(exporters, NewTransactionPostgresqlExporter(chain, table, nonce))
	}
	if config.Conf.Redis.Addr != "" {
		exporters = append(exporters, NewTransactionRedisExporter(chain, openAPIServer, nonce))
	}
	return exporters
}

func NewTransactionRedisExporter(chain, openAPIServer string, nonce uint64) Exporter {
	return &TransactionRedisExporter{Chain: chain, Nonce: nonce, OpenAPIServer: openAPIServer}
}

func (tre *TransactionRedisExporter) ExportItems(items any) {
	for _, item := range items.(model.Transactions) {
		if item.TxStatus != 0 {
			tre.handleItem(item)
			if item.ToAddress == nil && item.ContractAddress != "" && item.Nonce <= tre.Nonce {
				tre.appendItemToMessageQueue(item)
			} else {
				logrus.Infof("filter the address %s by policy: create contract with tx nonce less than %d", item.ContractAddress, tre.Nonce)
			}
		}
	}
}

func (tre *TransactionRedisExporter) handleItem(item model.Transaction) {
	key := fmt.Sprintf(TransactionAssociatedAddrs, tre.Chain)
	if item.Nonce > tre.Nonce {
		_, err := datastore.Redis().HDel(context.Background(), key, item.FromAddress).Result()
		if err != nil {
			logrus.Errorf("del %s in key %s from redis is err: %v", item.FromAddress, key, err)
		}
		return
	}
	isExist, err := datastore.Redis().HExists(context.Background(), key, item.FromAddress).Result()
	if err != nil {
		logrus.Errorf("get %s in key %s from redis is err: %v", item.FromAddress, TransactionAssociatedAddrs, err)
		return
	}
	addrs := []string{}
	if isExist {
		val, err := datastore.Redis().HGet(context.Background(), key, item.FromAddress).Result()
		if err != nil {
			logrus.Errorf("get %s in key %s from redis is err: %v", item.FromAddress, TransactionAssociatedAddrs, err)
			return
		}
		if val != "" {
			addrs = strings.Split(val, ",")
		}
	}
	if item.ToAddress == nil && item.ContractAddress != "" && item.Nonce <= tre.Nonce {
		addrs = append(addrs, item.ContractAddress)
		_, err = datastore.Redis().HSet(context.Background(), key, item.FromAddress, strings.Join(mapset.NewSet[string](addrs...).ToSlice(), ",")).Result()
		if err != nil {
			logrus.Errorf("set value %v to filed %s in key %s from redis is err: %v", addrs, item.FromAddress, key, err)
			return
		}
	}
}

func (tre *TransactionRedisExporter) appendItemToMessageQueue(item model.Transaction) {
	needFilter, err := tre.filterContractIsErc20OrErc721(item.ContractAddress)
	if err != nil {
		logrus.Errorf("filter contract is err: %v", err)
		return
	}
	if !needFilter {
		go func() {
			logrus.Infof("push contract %s to the get source queue", item.ContractAddress)
			time.Sleep(10 * time.Minute)
			contract, err := tre.getContractCode(item.ContractAddress)
			if err != nil {
				logrus.Errorf("get contract %s code is err: %v", item.ContractAddress, err)
				return
			}
			if contract.Result[0].SourceCode != "" {
				logrus.Infof("filter the address %s by policy: contract code is open source", item.ContractAddress)
				return
			}

			tx, err := tre.getSourceEthAddress(item.ContractAddress)
			if err != nil {
				logrus.Errorf("get contract %s's eth source is err: %v", item.ContractAddress, err)
				return
			}
			opcodes, err := tre.getOpcodes(tre.Chain, item.ContractAddress)
			if err != nil {
				logrus.Infof("get contract address %s's opcodes is err: %v", item.ContractAddress, err)
				return
			}
			fund := tx.Address
			if tx.Address != "" {
				if tx.Label != "" {
					fund = tx.Label
				}
			} else {
				fund = "scanError"
			}

			_, err = datastore.Redis().XAdd(context.Background(), &redis.XAddArgs{
				Stream: fmt.Sprintf("%s:v2", fmt.Sprintf(TransactionContractAddressStream, tre.Chain)),
				ID:     "*",
				Values: map[string]any{
					"chain":    tre.Chain,
					"txhash":   item.TxHash,
					"contract": item.ContractAddress,
					"fund":     fmt.Sprintf("%d-%s", len(tx.Nonce), fund),
					//"push4":    strings.Join(tre.GetContractPush4Args(opcodes), ","),
					"push20": strings.Join(tre.GetContractPush20Args(opcodes), ","),
				},
			}).Result()
			if err != nil {
				logrus.Errorf("send redis stream is err: %v", err)
				return
			}
			logrus.Infof("insert address %s to the redis mq: no match any filter policys", item.ContractAddress)
		}()
	} else {
		logrus.Infof("filter the address %s by policy: is erc20 or is erc721", item.ContractAddress)
	}
}

func (tre *TransactionRedisExporter) getSourceEthAddress(contractAddress string) (model.ScanTXResponse, error) {
	message := struct {
		Code int                  `json:"code"`
		Msg  string               `json:"msg"`
		Data model.ScanTXResponse `json:"data"`
	}{}
	url := fmt.Sprintf("%s/api/v1/address/%s/source_eth?apikey=%s&chain=%s", tre.OpenAPIServer, contractAddress, config.Conf.HTTPServer.APIKey, tre.Chain)
	resp, err := client.HTTPClient().Get(url)
	if err != nil {
		return model.ScanTXResponse{}, fmt.Errorf("get the contract source eth from etherscan is err: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.ScanTXResponse{}, fmt.Errorf("read response body is err :%v", err)
	}

	if err = json.Unmarshal(body, &message); err != nil {
		return model.ScanTXResponse{}, fmt.Errorf("json unmarshall from body %s is err: %v", string(body), err)
	}
	if message.Code != http.StatusOK {
		return model.ScanTXResponse{}, fmt.Errorf("get txs from open api server is err: %s", message.Msg)
	}

	return message.Data, nil
}

func (tre *TransactionRedisExporter) getContractCode(contractAddress string) (model.ScanContractResponse, error) {
	rand.Seed(time.Now().UnixNano())

	scanAPI := utils.GetScanAPI(tre.Chain)
	apiKeys := config.Conf.ScanInfos[tre.Chain].APIKeys

	scanAPIKey := apiKeys[rand.Intn(len(apiKeys))]
	contract := model.ScanContractResponse{}
	url := fmt.Sprintf("%s?module=contract&action=getsourcecode&address=%s&apikey=%s", scanAPI, contractAddress, scanAPIKey)
	resp, err := client.HTTPClient().Get(url)
	if err != nil {
		return contract, fmt.Errorf("get the contract source code from etherscan is err %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return contract, fmt.Errorf("read response body is err :%v", err)
	}

	if err = json.Unmarshal(body, &contract); err != nil {
		return contract, fmt.Errorf("json unmarshall from body %s is err %v", string(body), err)
	}

	if contract.Status != "1" {
		return contract, fmt.Errorf("get contract from scan is err %s", contract.Message)
	}

	return contract, nil
}

func (tre *TransactionRedisExporter) filterContractIsErc20OrErc721(address string) (bool, error) {
	code, err := client.EvmClient().CodeAt(context.Background(), common.HexToAddress(address), nil)
	if err != nil {
		return true, fmt.Errorf("failed to get byte code, got an err: %v", err)
	}
	if utils.IsErc20Or721(utils.Erc20Signatures, code, 5) ||
		utils.IsErc20Or721(utils.Erc721Signatures, code, 8) {
		return true, nil
	}
	return false, nil
}

func (tre *TransactionRedisExporter) getOpcodes(chain, address string) ([]string, error) {
	result := model.ScanStringResult{}
	return result.GetOpCodes(chain, address)
}

func (tre *TransactionRedisExporter) GetContractPush20Args(opcodes []string) []string {
	labelAddrs := []string{}
	args := []string{}
	for _, opcode := range opcodes {
		ops := strings.Split(opcode, " ")
		if len(ops) > 1 {
			if ops[0] == utils.PUSH20 && strings.ToLower(ops[1]) != utils.FFFFAddress {
				args = append(args, strings.ToLower(ops[1]))
			}
		}
	}
	addrs := mapset.NewSet[string](args...).ToSlice()
	if len(addrs) > 0 {
		labels := model.MetaDockLabelsResponse{}
		if err := labels.GetLabels(tre.Chain, addrs); err != nil {
			logrus.Errorf("get labels from metadocks in get opcode is err: %+v", err)
			return labelAddrs
		}
		labelMap := map[string]string{}
		for _, label := range labels {
			labelMap[label.Address] = label.Label
		}
		for _, addr := range addrs {
			value := addr
			if v, ok := labelMap[addr]; ok {
				value = v
			}
			labelAddrs = append(labelAddrs, value)
		}
	}
	return labelAddrs
}

func (tre *TransactionRedisExporter) GetContractPush4Args(opcodes []string) []string {
	args := []string{}
	for _, opcode := range opcodes {
		ops := strings.Split(opcode, " ")
		if len(ops) > 1 {
			if ops[0] == utils.PUSH4 && strings.ToLower(ops[1]) != utils.FFFFFunction {
				args = append(args, strings.ToLower(ops[1]))
			}
		}
	}
	return mapset.NewSet[string](args...).ToSlice()
}

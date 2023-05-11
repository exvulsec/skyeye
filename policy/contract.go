package policy

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
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/datastore"
	"go-etl/model"
	"go-etl/utils"
)

const (
	TransactionContractAddressStream = "%s:contract_address:stream"
)

func FilterContractByPolicy(chain, contractAddress string, txNonce, thresholdNonce uint64, interval int64, byteCode []byte) (int64, error) {
	if thresholdNonce > 0 && txNonce > thresholdNonce {
		return NoncePolicyDenied, nil
	}

	if len(byteCode) == 0 || len(byteCode[2:]) < 1000 {
		return ByteCodeLengthDenied, nil
	}

	isErc20OrErc721, err := filterContractIsErc20OrErc721(byteCode)
	if err != nil {
		return 0, fmt.Errorf("filter contract is erc20 or erc721 is err: %v", err)
	}
	if isErc20OrErc721 {
		return Erc20Erc721PolicyDenied, nil
	}
	logrus.Infof("push contract %s to the get source queue", contractAddress)
	if interval != 0 {
		time.Sleep(time.Duration(interval) * time.Minute)
	}
	contract, err := getContractCode(chain, contractAddress)
	if err != nil {
		return 0, fmt.Errorf("get contract %s code is err: %v", contractAddress, err)
	}
	if contract.Result[0].SourceCode != "" {
		return OpenSourceDenied, nil
	}
	return NoAnyDenied, nil
}

func SendItemToMessageQueue(chain, txhash, contractAddress, openApiServer string, code []byte, isNastiff bool) (map[string]any, error) {
	opcodes, err := getOpcodes(chain, contractAddress)
	if err != nil {
		return nil, fmt.Errorf("get contract address %s's opcodes is err: %v", contractAddress, err)
	}

	values := map[string]any{
		"chain":    utils.ConvertChainToDeFiHackLabChain(chain),
		"txhash":   txhash,
		"contract": contractAddress,
		//"push4":    strings.Join(tre.GetContractPush4Args(opcodes), ","),
		"push20":   strings.Join(getContractPush20Args(chain, opcodes), ","),
		"codeSize": len(code[2:]),
	}
	if isNastiff {
		scanTxResp, err := getSourceEthAddress(chain, contractAddress, openApiServer)
		if err != nil {
			return nil, fmt.Errorf("get contract %s's eth source is err: %v", contractAddress, err)
		}
		fund := scanTxResp.Address
		if scanTxResp.Address != "" {
			if scanTxResp.Label != "" {
				fund = fmt.Sprintf("%d-%s", len(scanTxResp.Nonce), scanTxResp.Label)
			}
		} else {
			fund = "0-scanError"
		}
		values["fund"] = fund
		_, err = datastore.Redis().XAdd(context.Background(), &redis.XAddArgs{
			Stream: fmt.Sprintf("%s:v2", fmt.Sprintf(TransactionContractAddressStream, chain)),
			ID:     "*",
			Values: values,
		}).Result()
		if err != nil {
			return nil, fmt.Errorf("send values to redis stream is err: %v", err)
		}
	}

	return values, nil
}

func getSourceEthAddress(chain, contractAddress, openApiServer string) (model.ScanTXResponse, error) {
	message := struct {
		Code int                  `json:"code"`
		Msg  string               `json:"msg"`
		Data model.ScanTXResponse `json:"data"`
	}{}
	url := fmt.Sprintf("%s/api/v1/address/%s/source_eth?apikey=%s&chain=%s", openApiServer, contractAddress, config.Conf.HTTPServer.APIKey, chain)
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

func getContractCode(chain, contractAddress string) (model.ScanContractResponse, error) {
	rand.Seed(time.Now().UnixNano())

	scanAPI := utils.GetScanAPI(chain)
	apiKeys := config.Conf.ScanInfos[chain].APIKeys

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

func filterContractIsErc20OrErc721(code []byte) (bool, error) {
	if utils.IsErc20Or721(utils.Erc20Signatures, code, 5) ||
		utils.IsErc20Or721(utils.Erc721Signatures, code, 8) {
		return true, nil
	}
	return false, nil
}

func getOpcodes(chain, address string) ([]string, error) {
	result := model.ScanStringResult{}
	return result.GetOpCodes(chain, address)
}

func getContractPush20Args(chain string, opcodes []string) []string {
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
		if err := labels.GetLabels(chain, addrs); err != nil {
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

func getContractPush4Args(opcodes []string) []string {
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

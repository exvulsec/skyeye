package model

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/utils"
)

type FilterPolicy interface {
	ApplyFilter(transaction NastiffTransaction) bool
}

type NonceFilter struct {
	ThresholdNonce uint64
}

func (nf *NonceFilter) ApplyFilter(tx NastiffTransaction) bool {
	if nf.ThresholdNonce > 0 && tx.Nonce > nf.ThresholdNonce {
		return true
	}
	return false
}

type ByteCodeFilter struct{}

func (bf *ByteCodeFilter) ApplyFilter(tx NastiffTransaction) bool {
	if len(tx.ByteCode) == 0 || len(tx.ByteCode[2:]) < 500 {
		return true
	}
	return false
}

type ContractTypeFilter struct{}

func (cf *ContractTypeFilter) ApplyFilter(tx NastiffTransaction) bool {
	if utils.IsErc20Or721(utils.Erc20Signatures, tx.ByteCode, utils.Erc20SignatureThreshold) ||
		utils.IsErc20Or721(utils.Erc721Signatures, tx.ByteCode, utils.Erc721SignatureThreshold) {
		return true
	}
	return false
}

type OpenSourceFilter struct {
	Interval int64
}

func (of *OpenSourceFilter) ApplyFilter(tx NastiffTransaction) bool {
	if err := GetDeDaubMd5(tx.Chain, tx.ContractAddress, tx.ByteCode); err != nil {
		logrus.Errorf("get dedaub md5 for %s on chain %s is err %v", tx.ContractAddress, tx.Chain, err)
	}
	if of.Interval != 0 {
		logrus.Infof("waiting contract %s is open source", tx.ContractAddress)
		time.Sleep(time.Duration(of.Interval) * time.Minute)
	}
	contract, err := GetContractCode(tx.Chain, tx.ContractAddress)
	if err != nil {
		logrus.Errorf("get contract %s code is err: %v", tx.ContractAddress, err)
		return true
	}
	if contract.Result[0].SourceCode != "" {
		return true
	}
	return false
}

func GetSourceEthAddress(chain, contractAddress, openApiServer string) (ScanTXResponse, error) {
	message := struct {
		Code int            `json:"code"`
		Msg  string         `json:"msg"`
		Data ScanTXResponse `json:"data"`
	}{}
	url := fmt.Sprintf("%s/api/v1/address/%s/source_eth?apikey=%s&chain=%s", openApiServer, contractAddress, config.Conf.HTTPServer.APIKey, chain)
	resp, err := client.HTTPClient().Get(url)
	if err != nil {
		return ScanTXResponse{}, fmt.Errorf("get the contract source eth from etherscan is err: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ScanTXResponse{}, fmt.Errorf("read response body is err :%v", err)
	}

	if err = json.Unmarshal(body, &message); err != nil {
		return ScanTXResponse{}, fmt.Errorf("json unmarshall from body %s is err: %v", string(body), err)
	}
	if message.Code != http.StatusOK {
		return ScanTXResponse{}, fmt.Errorf("get txs from open api server is err: %s", message.Msg)
	}

	return message.Data, nil
}

func GetContractCode(chain, contractAddress string) (ScanContractResponse, error) {
	rand.Seed(time.Now().UnixNano())

	scanAPI := utils.GetScanAPI(chain)
	apiKeys := config.Conf.ScanInfos[chain].APIKeys

	scanAPIKey := apiKeys[rand.Intn(len(apiKeys))]
	contract := ScanContractResponse{}
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

func GetOpcodes(chain, address string) ([]string, error) {
	result := ScanStringResult{}
	return result.GetOpCodes(chain, address)
}

func GetContractPush20Args(chain string, opcodes []string) []string {
	labelAddrs := []string{}
	noneLabelAddrs := []string{}
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
		labels := MetaDockLabelsResponse{}
		if err := labels.GetLabels(chain, addrs); err != nil {
			logrus.Errorf("get labels from metadocks in get opcode is err: %+v", err)
			return labelAddrs
		}
		labelMap := map[string]string{}
		for _, label := range labels {
			labelMap[label.Address] = label.Label
		}
		for _, addr := range addrs {
			if v, ok := labelMap[addr]; ok {
				labelAddrs = append(labelAddrs, v)
			} else {
				noneLabelAddrs = append(noneLabelAddrs, addr)
			}
		}
	}
	sort.SliceStable(labelAddrs, func(i, j int) bool {
		return labelAddrs[i] < labelAddrs[j]
	})
	if len(noneLabelAddrs) > 0 {
		labelAddrs = append(labelAddrs, fmt.Sprintf("0x{%d}", len(noneLabelAddrs)))
	}
	return labelAddrs
}

func GetContractPush4Args(opcodes []string) []string {
	args := []string{}
	for _, opcode := range opcodes {
		ops := strings.Split(opcode, " ")
		if len(ops) > 1 {
			if ops[0] == utils.PUSH4 && strings.ToLower(ops[1]) != utils.FFFFFunction {
				args = append(args, strings.ToLower(ops[1]))
			}
		}
	}
	signatures := mapset.NewSet[string](args...).ToSlice()
	textSignatures, err := GetSignatures(signatures)
	if err != nil {
		logrus.Errorf("get signature is err %v", err)
		return []string{}
	}
	return textSignatures
}

func GetDeDaubMd5(chain, address string, byteCode []byte) error {
	d := DeDaub{
		Chain:   chain,
		Address: address,
	}
	if err := d.Get(); err != nil {
		return fmt.Errorf("get chain %s address %s from db is err %v", chain, address, err)
	}
	if d.MD5 != "" && len(d.MD5) == 32 {
		return nil
	}

	var drs DeDaubResponseString
	if err := drs.GetCodeMD5(byteCode); err != nil {
		return fmt.Errorf("get code md5 for %s is err %v", address, err)
	}

	d.MD5 = strings.Trim(string(drs), `"`)
	if err := d.Create(); err != nil {
		return fmt.Errorf("insert chain %s address %s md5 %s to db is err %v", chain, address, drs, err)
	}
	return nil

}

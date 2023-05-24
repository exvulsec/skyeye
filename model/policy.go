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
	"github.com/ethereum/go-ethereum/core/asm"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/utils"
)

type FilterPolicy interface {
	ApplyFilter(transaction *NastiffTransaction) bool
}

type NonceFilter struct {
	ThresholdNonce uint64
}

func (nf *NonceFilter) ApplyFilter(tx *NastiffTransaction) bool {
	if nf.ThresholdNonce > 0 && tx.Nonce > nf.ThresholdNonce {
		return true
	}
	return false
}

type ByteCodeFilter struct{}

func (bf *ByteCodeFilter) ApplyFilter(tx *NastiffTransaction) bool {
	if len(tx.ByteCode) == 0 || len(tx.ByteCode[2:]) < 500 {
		return true
	}
	return false
}

type ContractTypeFilter struct{}

func (cf *ContractTypeFilter) ApplyFilter(tx *NastiffTransaction) bool {
	opCodeArgs := GetPushTypeArgs(tx.ByteCode)
	push4Codes := opCodeArgs[utils.PUSH4]
	push20Codes := opCodeArgs[utils.PUSH20]
	tx.Push4Args = GetPush4Args(push4Codes)
	tx.Push20Args = GetPush20Args(tx.Chain, push20Codes)

	if utils.IsErc20Or721(utils.Erc20Signatures, push4Codes, utils.Erc20SignatureThreshold) ||
		utils.IsErc20Or721(utils.Erc721Signatures, push4Codes, utils.Erc721SignatureThreshold) {
		return true
	}
	return false
}

type Push4ArgsFilter struct{}

func (p4 *Push4ArgsFilter) ApplyFilter(tx *NastiffTransaction) bool {
	return false
}

type Push20ArgsFilter struct{}

func (p20 *Push20ArgsFilter) ApplyFilter(tx *NastiffTransaction) bool {
	return len(tx.Push20Args) == 0
}

type OpenSourceFilter struct {
	Interval int
}

func (of *OpenSourceFilter) ApplyFilter(tx *NastiffTransaction) bool {
	if err := GetDeDaubMd5(tx.Chain, tx.ContractAddress, tx.ByteCode); err != nil {
		logrus.Errorf("get dedaub md5 for %s on chain %s is err %v", tx.ContractAddress, tx.Chain, err)
	}
	if of.Interval != 0 {
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

func GetPushTypeArgs(byteCode []byte) map[string][]string {
	args := map[string][]string{}
	if len(byteCode) != 0 {
		byteCode = byteCode[2:]
		it := asm.NewInstructionIterator(byteCode)
		l1 := vm.INVALID
		l2 := vm.INVALID
		arg := []byte{}
		opCodeArg := ""
		for it.Next() {
			if fmt.Sprintf("opcode %#x not defined", int(it.Op())) == it.Op().String() {
				break
			}
			if l2 == vm.DUP1 && l1 == vm.PUSH4 && it.Op() == vm.EQ {
				opCodeArg = fmt.Sprintf("%#x", arg)
				if opCodeArg != utils.FFFFFunction {
					args[utils.PUSH4] = append(args[utils.PUSH4], opCodeArg)
				}
			} else if it.Op() == vm.PUSH4 {
				arg = it.Arg()
			} else if it.Op() == vm.PUSH20 {
				opCodeArg = fmt.Sprintf("%#x", it.Arg())
				if opCodeArg != utils.FFFFAddress {
					args[utils.PUSH20] = append(args[utils.PUSH20], opCodeArg)
				}

			}
			l2 = l1
			l1 = it.Op()

		}
		return args
	}
	return map[string][]string{
		utils.PUSH20: []string{},
		utils.PUSH4:  []string{},
	}
}

func GetPush4Args(args []string) []string {
	byteSignatures := mapset.NewSet[string](args...).ToSlice()
	textSignatures, err := GetSignatures(byteSignatures)
	if err != nil {
		logrus.Errorf("get signature is err %v", err)
		return []string{}
	}
	return textSignatures
}

func GetPush20Args(chain string, args []string) []string {
	labelAddrs := []string{}
	noneLabelAddrs := []string{}
	addrs := mapset.NewSet[string](args...).ToSlice()
	if len(addrs) > 0 {
		labels := MetaDockLabelsResponse{}
		if err := labels.GetLabels(chain, addrs); err != nil {
			logrus.Errorf("get labels from metadocks in get opcode is err: %+v", err)
			return labelAddrs
		}
		labelMap := map[string]string{}
		for _, label := range labels {
			if label.Label != "" {
				labelMap[label.Address] = label.Label
			}
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

package model

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
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

type PolicyCalc interface {
	Calc(transaction *NastiffTransaction) int
}

type NoncePolicyCalc struct{}

func (npc *NoncePolicyCalc) Calc(tx *NastiffTransaction) int {
	if tx.Nonce >= 50 {
		return 0
	}
	if 10 <= tx.Nonce && tx.Nonce < 50 {
		return 5 - (int(tx.Nonce)-10)/10
	}
	return 10 - int(tx.Nonce)
}

type ByteCodePolicyCalc struct{}

func (bpc *ByteCodePolicyCalc) Calc(tx *NastiffTransaction) int {
	if len(tx.ByteCode) == 0 || len(tx.ByteCode[2:]) < 500 {
		return 0
	}
	return 12
}

type ContractTypePolicyCalc struct{}

func (cpc *ContractTypePolicyCalc) Calc(tx *NastiffTransaction) int {
	opCodeArgs := GetPushTypeArgs(tx.ByteCode)
	push4Codes := opCodeArgs[utils.PUSH4]
	push20Codes := opCodeArgs[utils.PUSH20]
	tx.Push4Args = GetPush4Args(push4Codes)
	tx.Push20Args = GetPush20Args(tx.Chain, push20Codes)

	if utils.IsErc20Or721(utils.Erc20Signatures, push4Codes, utils.Erc20SignatureThreshold) ||
		utils.IsErc20Or721(utils.Erc721Signatures, push4Codes, utils.Erc721SignatureThreshold) {
		return 0
	}
	return 20
}

type Push4PolicyCalc struct {
	FlashLoanFuncNames []string
}

func (p4pc *Push4PolicyCalc) Calc(tx *NastiffTransaction) int {
	if tx.hasFlashLoan(p4pc.FlashLoanFuncNames) {
		return 50
	}
	return 0
}

type Push20PolicyCalc struct{}

func (p20pc *Push20PolicyCalc) Calc(tx *NastiffTransaction) int {
	if len(tx.Push20Args) == 0 {
		return 0
	}
	return 2
}

type OpenSourcePolicyCalc struct {
}

func (opc *OpenSourcePolicyCalc) Calc(tx *NastiffTransaction) int {
	contract, err := GetContractCodeFromScan(tx.Chain, tx.ContractAddress)
	if err != nil {
		logrus.Errorf("getting code from the %s scan for the contract %s is err: %v", tx.Chain, tx.ContractAddress, err)
		return 0
	}
	if contract.Result[0].SourceCode != "" {
		return 0
	}
	return 25
}

type FundPolicyCalc struct {
	IsNastiff     bool
	OpenAPIServer string
}

func (fpc *FundPolicyCalc) Calc(tx *NastiffTransaction) int {
	if fpc.IsNastiff {
		var fund string
		scanTxResp, err := GetFundAddress(tx.Chain, tx.FromAddress, fpc.OpenAPIServer)
		if err != nil {
			logrus.Errorf("get contract %s's fund is err: %v", tx.ContractAddress, err)
		}
		if scanTxResp.Address != "" {
			label := scanTxResp.Label
			if label == "" {
				if len(scanTxResp.Nonce) == 5 {
					label = "UnKnown"
				} else {
					label = scanTxResp.Address
				}
			}
			fund = fmt.Sprintf("%d-%s", len(scanTxResp.Nonce), label)
		} else {
			fund = "0-scanError"
		}
		tx.Fund = fund

	}
	switch {
	case strings.Contains(strings.ToLower(tx.Fund), strings.ToLower(TornadoCash)):
		return 40
	case strings.Contains(strings.ToLower(tx.Fund), strings.ToLower(ChangeNow)):
		return 13
	default:
		return 0
	}
}

func GetFundAddress(chain, contractAddress, openApiServer string) (ScanTXResponse, error) {
	message := struct {
		Code int            `json:"code"`
		Msg  string         `json:"msg"`
		Data ScanTXResponse `json:"data"`
	}{}
	url := fmt.Sprintf("%s/api/v1/address/%s/fund?apikey=%s&chain=%s", openApiServer, contractAddress, config.Conf.HTTPServer.APIKey, chain)
	resp, err := client.HTTPClient().Get(url)
	if err != nil {
		return ScanTXResponse{}, fmt.Errorf("get the contract fund from scan is err: %v", err)
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

func GetContractCodeFromScan(chain, contractAddress string) (ScanContractResponse, error) {
	rand.Seed(time.Now().UnixNano())

	scanAPI := utils.GetScanAPI(chain)
	apiKeys := config.Conf.ScanInfos[chain].APIKeys

	scanAPIKey := apiKeys[rand.Intn(len(apiKeys))]
	contract := ScanContractResponse{}
	url := fmt.Sprintf("%s?module=contract&action=getsourcecode&address=%s&apikey=%s", scanAPI, contractAddress, scanAPIKey)
	resp, err := client.HTTPClient().Get(url)
	if err != nil {
		return contract, fmt.Errorf("get the contract source code from scan %s is err %v", scanAPI, err)
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

	for index := range textSignatures {
		textSignature := textSignatures[index]
		texts := strings.Split(textSignature, "(")
		if len(texts) > 0 {
			textSignatures[index] = texts[0]
		}
	}

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
		addrLabels := []AddressLabel{}
		for _, addr := range addrs {
			label := AddressLabel{
				Chain:   chain,
				Address: addr,
			}
			if err := label.GetLabel(chain, addr); err != nil {
				logrus.Errorf("get address %s label is err %v", addr, err)
			}
			addrLabels = append(addrLabels, label)
		}
		for _, label := range addrLabels {
			if label.Label != "" {
				labelAddrs = append(labelAddrs, label.Label)
			} else {
				noneLabelAddrs = append(noneLabelAddrs, label.Address)
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

func LoadFlashLoanFuncNames() []string {
	funcNameList := []string{}
	f, err := os.Open(config.Conf.ETL.FlashLoanFile)
	if err != nil {
		logrus.Fatalf("read flash loan config file %s is err %v", config.Conf.ETL.FlashLoanFile, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() != "" {
			funcNames := strings.Split(scanner.Text(), "(")
			if len(funcNames) > 0 {
				funcNameList = append(funcNameList, funcNames[0])
			}
		}
	}

	if err := scanner.Err(); err != nil {
		logrus.Fatalf("read flash loan function names is err %v", err)
	}

	return mapset.NewSet[string](funcNameList...).ToSlice()
}

package model

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/asm"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/utils"
)

type PolicyCalc interface {
	Calc(transaction *SkyEyeTransaction) int
	Name() string
}

type MultiContractCalc struct {
}

func (mcc *MultiContractCalc) Calc(tx *SkyEyeTransaction) int {
	txTraces := GetTransactionTrace(tx.TxHash)
	contractAddrs := []string{}
	for _, txTrace := range txTraces {
		switch txTrace.To {
		case "":
			contractAddrs = append(contractAddrs, txTrace.ContractAddress)
		default:
			if IsInContractAddrs(contractAddrs, txTrace.To) && IsInContractAddrs(contractAddrs, txTrace.From) && txTrace.Input != "0x" {
				tx.IsMultiContract = true
				input := txTrace.Input[:10]
				s := Signature{
					ByteSign: input,
				}
				if err := s.GetTextSign(); err != nil {
					logrus.Error(err)
					return 0
				}
				if s.TextSign != "" {
					return 0
				}
				return 60
			}
		}
	}
	return 0
}

func IsInContractAddrs(contracts []string, toAddr string) bool {
	for _, contract := range contracts {
		if contract == toAddr {
			return true
		}
	}
	return false
}

func (mcc *MultiContractCalc) Name() string {
	return "MultiContract"
}

type NoncePolicyCalc struct{}

func (npc *NoncePolicyCalc) Calc(tx *SkyEyeTransaction) int {
	if tx.Nonce >= 50 {
		return 0
	}
	if 10 <= tx.Nonce && tx.Nonce < 50 {
		return 5 - (int(tx.Nonce)-10)/10
	}
	return 10 - int(tx.Nonce)
}
func (npc *NoncePolicyCalc) Name() string {
	return "Nonce"
}

type ByteCodePolicyCalc struct{}

func (bpc *ByteCodePolicyCalc) Calc(tx *SkyEyeTransaction) int {
	if len(tx.ByteCode) == 0 || len(tx.ByteCode[2:]) < 500 {
		return 0
	}
	return 12
}

func (bpc *ByteCodePolicyCalc) Name() string {
	return "ByteCode"
}

type ContractTypePolicyCalc struct{}

func (cpc *ContractTypePolicyCalc) Calc(tx *SkyEyeTransaction) int {
	opCodeArgs := GetPushTypeArgs(tx.ByteCode)
	push4Codes := opCodeArgs[utils.PUSH4]
	push20Codes := opCodeArgs[utils.PUSH20]
	stringLogs := opCodeArgs[utils.LOGS]
	tx.Push4Args = GetPush4Args(push4Codes)
	tx.Push20Args = GetPush20Args(tx.Chain, push20Codes)
	tx.PushStringLogs = stringLogs

	if utils.IsErc20Or721(utils.Erc20Signatures, push4Codes, utils.Erc20SignatureThreshold) ||
		utils.IsErc20Or721(utils.Erc721Signatures, push4Codes, utils.Erc721SignatureThreshold) {
		return 0
	}
	return 20
}
func (cpc *ContractTypePolicyCalc) Name() string {
	return "ContractType"
}

type Push4PolicyCalc struct {
	FlashLoanFuncNames []string
}

func (p4pc *Push4PolicyCalc) Calc(tx *SkyEyeTransaction) int {
	if tx.hasFlashLoan(p4pc.FlashLoanFuncNames) {
		return 30
	}
	return 0
}
func (p4pc *Push4PolicyCalc) Name() string {
	return "Push4"
}

type Push20PolicyCalc struct{}

func (p20pc *Push20PolicyCalc) Calc(tx *SkyEyeTransaction) int {
	if len(tx.Push20Args) == 0 {
		return 0
	}
	if tx.hasRiskAddress([]string{"PancakeSwap: Router v2"}) {
		return 10
	}
	return 5
}
func (p20pc *Push20PolicyCalc) Name() string {
	return "Push20"
}

type FundPolicyCalc struct {
	IsNastiff bool
}

func (fpc *FundPolicyCalc) Calc(tx *SkyEyeTransaction) int {
	if fpc.IsNastiff {
		var fund string
		scanTxResp, err := fpc.SearchFund(tx.Chain, tx.FromAddress)
		if err != nil {
			logrus.Errorf("get contract %s's fund is err: %v", tx.ContractAddress, err)
		}
		if scanTxResp.Address != "" {
			label := scanTxResp.Label
			if scanTxResp.Address != utils.ScanGenesisAddress {
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
	case strings.Contains(strings.ToLower(tx.Fund), strings.ToLower(FixedFloat)):
		return 20
	case strings.Contains(strings.ToLower(tx.Fund), strings.ToLower(ChangeNow)):
		return 13
	default:
		return 0
	}
}
func (fpc *FundPolicyCalc) Name() string {
	return "Fund"
}

func (fpc *FundPolicyCalc) SearchFund(chain, address string) (ScanTXResponse, error) {
	txResp := ScanTXResponse{}
	scanAPI := fmt.Sprintf("%s%s", utils.GetScanAPI(chain), utils.APIQuery)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		scanInfo := config.Conf.ScanInfos[chain]
		index := r.Intn(len(scanInfo.APIKeys))
		scanAPIKEY := scanInfo.APIKeys[index]
		apis := []string{
			fmt.Sprintf(scanAPI, scanAPIKEY, address, utils.ScanTransactionAction),
			fmt.Sprintf(scanAPI, scanAPIKEY, address, utils.ScanTraceAction),
		}
		var (
			transaction ScanTransaction
			trace       ScanTransaction
		)

		for _, api := range apis {
			resp, err := client.HTTPClient().Get(api)
			if err != nil {
				return txResp, fmt.Errorf("get address %s's from scan api is err %v", address, err)
			}
			defer resp.Body.Close()
			base := ScanBaseResponse{}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return txResp, fmt.Errorf("read body from resp.Body via %s  is err %v", api, err)
			}
			if err = json.Unmarshal(body, &base); err != nil {
				return txResp, fmt.Errorf("unmarshal json from body to scan base response via %s is err %v", api, err)
			}
			if base.Message == "NOTOK" {
				result := ScanStringResult{}
				if err = json.Unmarshal(body, &result); err != nil {
					return txResp, fmt.Errorf("unmarshal json from body to scan string result via %s is err %v", api, err)
				}
				return txResp, fmt.Errorf("get address %s from scan via %s is err: %s, message is %s", address, api, err, result.Message)
			}
			tx := ScanTransactionResponse{}
			if err = json.Unmarshal(body, &tx); err != nil {
				return txResp, fmt.Errorf("unmarshal json from body to scan transaction response via api %s is err %v", api, err)
			}
			if len(tx.Result) > 0 {
				if err = tx.Result[0].ConvertStringToInt(); err != nil {
					return txResp, fmt.Errorf("convert string to int is err: %v", err)
				}
				if strings.Contains(api, utils.ScanTraceAction) {
					trace = tx.Result[0]
				} else {
					transaction = tx.Result[0]
				}
			}
		}
		if transaction.FromAddress == "" && trace.FromAddress != "" {
			address = trace.FromAddress
		} else {
			address = transaction.FromAddress
			if transaction.Timestamp > trace.Timestamp && trace.Timestamp > 0 {
				address = trace.FromAddress
			}
		}

		var (
			nonce uint64
			err   error
		)

		if address != "" {
			nonce, err = client.MultiEvmClient()[chain].PendingNonceAt(context.Background(), common.HexToAddress(address))
			if err != nil {
				return txResp, fmt.Errorf("get nonce for address %s is err: %v", address, err)
			}
			txResp.Nonce = append(txResp.Nonce, nonce)
		}
		var addrLabel = AddressLabel{Label: utils.ScanGenesisAddress}
		if address != utils.ScanGenesisAddress && address != "" {
			if err = addrLabel.GetLabel(chain, address); err != nil {
				return txResp, fmt.Errorf("get address %s label is err: %v", address, err)
			}
		}

		if addrLabel.IsTornadoCashAddress() ||
			addrLabel.IsFixedFloat() ||
			addrLabel.IsChangeNow() ||
			address == "" ||
			address == utils.ScanGenesisAddress ||
			len(txResp.Nonce) == 5 {

			txResp.Address = address
			txResp.Label = addrLabel.Label
			break
		}
	}
	return txResp, nil
}
func IsPrintableASCII(r rune) bool {
	return r >= 32 && r <= 126
}

func GetPushTypeArgs(byteCode []byte) map[string][]string {
	args := map[string][]string{}
	if len(byteCode) > 2 {
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
			} else if it.Op() == vm.PUSH10 || it.Op() == vm.PUSH14 ||
				it.Op() == vm.PUSH15 || it.Op() == vm.PUSH16 ||
				it.Op() == vm.PUSH18 || it.Op() == vm.PUSH22 ||
				it.Op() == vm.PUSH32 {
				arg = it.Arg()
				var isASCII = true
				for _, char := range arg {
					if !IsPrintableASCII(rune(char)) {
						isASCII = false
						break
					}
				}
				if isASCII {
					args[utils.LOGS] = append(args[utils.LOGS], string(arg))
				}
			}

			l2 = l1
			l1 = it.Op()

		}
		return args
	}
	return map[string][]string{
		utils.PUSH20: {},
		utils.PUSH4:  {},
		utils.LOGS:   {},
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

package model

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ethereum/go-ethereum/core/asm"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/utils"
)

var FuncNameList []string

func init() {
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

	FuncNameList = mapset.NewSet[string](funcNameList...).ToSlice()
}

type PolicyCalc interface {
	Calc(transaction *SkyEyeTransaction) int
	Name() string
	Filter(tx *SkyEyeTransaction) bool
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
				isASCII := true
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

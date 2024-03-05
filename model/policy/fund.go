package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/model"
	"go-etl/utils"
)

type FundPolicyCalc struct {
	IsNastiff bool
}

func (fpc *FundPolicyCalc) Calc(tx *model.SkyEyeTransaction) int {
	if fpc.IsNastiff {
		var fund string
		if tx.Chain == utils.ChainAvalanche {
			return 0
		}
		scanTxResp, err := fpc.SearchFund(tx.Chain, tx.FromAddress)
		if err != nil {
			logrus.Errorf("get contract %s's fund is err: %v", tx.ContractAddress, err)
		}
		if scanTxResp.Address != "" {
			label := scanTxResp.Label
			if scanTxResp.Address != utils.ScanGenesisAddress {
				if len(scanTxResp.Nonce) == 5 {
					label = "UnKnown"
				} else if scanTxResp.Label != utils.ScanGenesisAddress {
					label = scanTxResp.Label
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
	case strings.Contains(strings.ToLower(tx.Fund), strings.ToLower(model.TornadoCash)):
		return 40
	case strings.Contains(strings.ToLower(tx.Fund), strings.ToLower(model.FixedFloat)):
		return 20
	case strings.Contains(strings.ToLower(tx.Fund), strings.ToLower(model.ChangeNow)):
		return 13
	default:
		return 0
	}
}

func (fpc *FundPolicyCalc) Name() string {
	return "Fund"
}

func (fpc *FundPolicyCalc) SearchFund(chain, address string) (model.ScanTXResponse, error) {
	txResp := model.ScanTXResponse{}
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
			transaction model.ScanTransaction
			trace       model.ScanTransaction
		)

		for _, api := range apis {
			resp, err := client.HTTPClient().Get(api)
			if err != nil {
				return txResp, fmt.Errorf("get address %s's from scan api is err %v", address, err)
			}
			defer resp.Body.Close()
			base := model.ScanBaseResponse{}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return txResp, fmt.Errorf("read body from resp.Body via %s  is err %v", api, err)
			}
			if err = json.Unmarshal(body, &base); err != nil {
				return txResp, fmt.Errorf("unmarshal json from body to scan base response via %s is err %v", api, err)
			}
			if base.Message == "NOTOK" {
				result := model.ScanStringResult{}
				if err = json.Unmarshal(body, &result); err != nil {
					return txResp, fmt.Errorf("unmarshal json from body to scan string result via %s is err %v", api, err)
				}
				return txResp, fmt.Errorf("get address %s from scan via %s is err: %s, message is %s", address, api, err, result.Message)
			}
			tx := model.ScanTransactionResponse{}
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
		addrLabel := model.AddressLabel{Label: utils.ScanGenesisAddress}
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

func (fpc *FundPolicyCalc) Filter(tx *model.SkyEyeTransaction) bool {
	return false
}

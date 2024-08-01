package model

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/utils"
)

type FundPolicyCalc struct {
	NeedFund bool
	Chain    string
}

func (fpc *FundPolicyCalc) Calc(tx *SkyEyeTransaction) int {
	if fpc.NeedFund {
		var fund string
		if fpc.Chain == utils.ChainAvalanche {
			return 0
		}
		scanTxResp, err := fpc.SearchFund(fpc.Chain, tx.FromAddress)
		if err != nil {
			logrus.Errorf("get contract %s's fund is err: %v", tx.ContractAddress, err)
		}
		if scanTxResp.Address != "" {
			fund = scanTxResp.Label
		} else {
			fund = "unknown"
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

func (fpc *FundPolicyCalc) GetTransactionInfoFromScan(url string) (ScanTransactionResponse, error) {
	txResp := ScanTransactionResponse{}

	resp, err := client.HTTPClient().Get(url)
	if err != nil {
		return txResp, fmt.Errorf("get scan resp from %s is err %v", url, err)
	}
	defer resp.Body.Close()
	base := ScanBaseResponse{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return txResp, fmt.Errorf("read body from resp.Body via %s is err %v", url, err)
	}
	if err = json.Unmarshal(body, &base); err != nil {
		return txResp, fmt.Errorf("unmarshal json from body to scan base response via %s is err %v", url, err)
	}
	if base.Message == "NOTOK" {
		result := ScanStringResult{}
		if err = json.Unmarshal(body, &result); err != nil {
			return txResp, fmt.Errorf("unmarshal json from body to scan string result via %s is err %v", url, err)
		}
		return txResp, fmt.Errorf("get scan info via %s is err: %s, message is %s", url, err, result.Message)
	}

	if err = json.Unmarshal(body, &txResp); err != nil {
		return txResp, fmt.Errorf("unmarshal json from body to scan transaction response via api %s is err %v", url, err)
	}
	return txResp, nil
}

func (fpc *FundPolicyCalc) GetFundAddress(chain, address string) (string, error) {
	scanAPI := fmt.Sprintf("%s%s", utils.GetScanAPI(chain), utils.APIQuery)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	scanInfo := config.Conf.ScanInfos[chain]
	index := r.Intn(len(scanInfo.APIKeys))
	scanAPIKEY := scanInfo.APIKeys[index]
	var (
		transaction *ScanTransaction
		trace       *ScanTransaction
	)

	transactionResp, err := fpc.GetTransactionInfoFromScan(fmt.Sprintf(scanAPI, scanAPIKEY, address, utils.ScanTransactionAction))
	if err != nil {
		return "", err
	}
	traceResp, err := fpc.GetTransactionInfoFromScan(fmt.Sprintf(scanAPI, scanAPIKEY, address, utils.ScanTraceAction))
	if err != nil {
		return "", err
	}
	if len(transactionResp.Result) > 0 {
		if err := transactionResp.Result[0].ConvertStringToInt(); err != nil {
			return "", fmt.Errorf("convert string to int is err: %v", err)
		}
		transaction = &transactionResp.Result[0]
	}
	if len(traceResp.Result) > 0 {
		if err := traceResp.Result[0].ConvertStringToInt(); err != nil {
			return "", fmt.Errorf("convert string to int is err: %v", err)
		}
		trace = &traceResp.Result[0]
	}
	var fundAddress string
	if trace == nil && transaction == nil {
		return "", nil
	}

	if transaction != nil && (trace == nil || transaction.Timestamp < trace.Timestamp || trace.Timestamp == 0) {
		fundAddress = transaction.FromAddress
	} else {
		fundAddress = trace.FromAddress
	}
	return fundAddress, nil
}

func (fpc *FundPolicyCalc) SearchFund(chain, address string) (ScanTXResponse, error) {
	txResp := ScanTXResponse{}
	searchAddress := address
	for i := 0; i < 3; i++ {
		fundAddress, err := fpc.GetFundAddress(chain, searchAddress)
		if err != nil {
			return txResp, err
		}
		if fundAddress == "" {
			return txResp, nil
		}

		addrLabel := AddressLabel{Label: fundAddress}

		if fundAddress != utils.ScanGenesisAddress {
			if err := addrLabel.GetLabel(chain, fundAddress); err != nil {
				return txResp, fmt.Errorf("get address %s label is err: %v", address, err)
			}
		}

		if addrLabel.IsTornadoCashAddress() ||
			addrLabel.IsFixedFloat() ||
			addrLabel.IsChangeNow() ||
			fundAddress == utils.ScanGenesisAddress || i == 2 {

			txResp.Address = fundAddress
			txResp.Label = addrLabel.Label
			return txResp, nil
		}
		searchAddress = fundAddress
	}
	return txResp, nil
}

func (fpc *FundPolicyCalc) Filter(tx *SkyEyeTransaction) bool {
	return false
}

func (fpc *FundPolicyCalc) GetAddressTransactionGraph(chain, address string) (*Graph, error) {
	scanAPI := fmt.Sprintf("%s%s", utils.GetScanAPI(chain), utils.APIQuery)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	scanInfo := config.Conf.ScanInfos[chain]

	actions := []string{utils.ScanTransactionAction, utils.ScanTokenTransactionAction, utils.ScanNFTTransactionAction}

	wg := sync.WaitGroup{}
	txs := []ScanTransaction{}
	for _, action := range actions {
		wg.Add(1)
		go func() {
			defer wg.Done()

			index := r.Intn(len(scanInfo.APIKeys))
			scanAPIKEY := scanInfo.APIKeys[index]
			resp, err := fpc.GetTransactionInfoFromScan(fmt.Sprintf(scanAPI, scanAPIKEY, address, action))
			if err != nil {
				logrus.Errorf("get %s resp from scan is err %v", action, err)
				return
			}
			txs = append(txs, resp.Result...)
		}()

	}
	wg.Wait()
	return NewGraphFromScan(txs), nil
}

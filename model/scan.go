package model

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/utils"
)

type ScanBaseResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ScanStringResult struct {
	ScanBaseResponse
	Result string `json:"result"`
}

type ScanContractResponse struct {
	ScanBaseResponse
	Result []ScanContract `json:"result"`
}

type ScanContract struct {
	SourceCode   string `json:"SourceCode"`
	ABI          string `json:"ABI"`
	ContractName string `json:"ContractName"`
}

type ScanTransactionResponse struct {
	ScanBaseResponse
	Result []ScanTransaction `json:"result"`
}

type ScanTransaction struct {
	TimestampString string `json:"timeStamp"`
	Timestamp       int64  `json:"-"`
	FromAddress     string `json:"from"`
}

type ScanTXResponse struct {
	Address string `json:"address"`
	Label   string `json:"label"`
}

func (st *ScanTransaction) ConvertStringToInt() error {
	timestamp, err := strconv.ParseInt(st.TimestampString, 10, 64)
	if err != nil {
		return fmt.Errorf("convert timestamp %s to int64 is err: %v", st.TimestampString, err)
	}
	st.Timestamp = timestamp
	return nil
}

func (rs *ScanStringResult) GetOpCodes(chain, address string) (string, error) {
	scanURL := utils.GetScanURL(chain)

	headers := map[string]string{
		"authority":          "etherscan.io",
		"accept":             "application/json, text/javascript, */*; q=0.01",
		"accept-language":    "zh,zh-CN;q=0.9",
		"referer":            fmt.Sprintf("%s/address/%s", scanURL, address),
		"sec-ch-ua":          `"Chromium";v="112", "Google Chrome";v="112", "Not:A-Brand";v="99"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": "macOS",
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-origin",
		"user-agent":         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36",
		"x-requested-with":   "XMLHttpRequest",
	}

	scanAPIURL := fmt.Sprintf("%s/api?", scanURL)

	params := map[string]string{
		"module":  "opcode",
		"action":  "getopcode",
		"address": address,
	}
	req, err := http.NewRequest(http.MethodGet, scanAPIURL, nil)
	if err != nil {
		return "", fmt.Errorf("new request for get meta dock labels is err: %v", err)
	}

	q := req.URL.Query()

	for key, value := range params {
		q.Set(key, value)
	}
	req.URL.RawQuery = q.Encode()

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("receive response from %s is err: %v", scanAPIURL, err)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read data from resp.Body is err: %v", err)
	}
	defer resp.Body.Close()
	if err = json.Unmarshal(data, rs); err != nil {
		return "", fmt.Errorf("unmarshall data %s is err: %v", string(data), err)
	}
	return rs.Result, nil
}

package model

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
	"go-etl/utils"
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
	Address string   `json:"address"`
	Label   string   `json:"label"`
	Nonce   []uint64 `json:"nonce"`
}

func (st *ScanTransaction) ConvertStringToInt() error {
	timestamp, err := strconv.ParseInt(st.TimestampString, 10, 64)
	if err != nil {
		return fmt.Errorf("convert timestamp %s to int64 is err: %v", st.TimestampString, err)
	}
	st.Timestamp = timestamp
	return nil
}

func (st *ScanTXResponse) IsCEX() bool {
	for _, cex := range config.Conf.ETLConfig.Cexs {
		if strings.HasPrefix(strings.ToLower(st.Label), strings.ToLower(cex)) {
			return true
		}
	}
	return false
}

func (rs *ScanStringResult) GetPush20OpCode(chain, address string) ([]string, error) {
	addrs := []string{}
	headers := map[string]string{
		"authority":          "etherscan.io",
		"accept":             "application/json, text/javascript, */*; q=0.01",
		"accept-language":    "zh,zh-CN;q=0.9",
		"referer":            fmt.Sprintf("https://etherscan.io/address/%s", address),
		"sec-ch-ua":          `"Chromium";v="112", "Google Chrome";v="112", "Not:A-Brand";v="99"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": "macOS",
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-origin",
		"user-agent":         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36",
		"x-requested-with":   "XMLHttpRequest",
	}

	scanURL := "https://etherscan.io/api?"

	params := map[string]string{
		"module":  "opcode",
		"action":  "getopcode",
		"address": address,
	}
	req, err := http.NewRequest(http.MethodGet, scanURL, nil)
	if err != nil {

		return addrs, fmt.Errorf("new request for get meta dock labels is err: %v", err)
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
		return addrs, fmt.Errorf("receive response from %s is err: %v", scanURL, err)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return addrs, fmt.Errorf("read data from resp.Body is err: %v", err)
	}
	defer resp.Body.Close()
	if err = json.Unmarshal(data, rs); err != nil {
		return addrs, fmt.Errorf("unmarshall data %s is err: %v", string(data), err)
	}
	opcodes := strings.Split(rs.Result, "<br>")
	for _, opcode := range opcodes {
		ops := strings.Split(opcode, " ")
		if len(ops) > 1 {
			if ops[0] == utils.PUSH20 && strings.ToLower(ops[1]) != utils.FFFFAddress {
				addrs = append(addrs, strings.ToLower(ops[1]))
			}
		}
	}
	addrs = mapset.NewSet[string](addrs...).ToSlice()
	labelAddrs := []string{}
	if len(addrs) > 0 {
		labels := MetaDockLabelsResponse{}
		if err = labels.GetLabels(chain, addrs); err != nil {
			logrus.Errorf("get labels from metadocks in get opcode is err: %+v", err)
			return addrs, nil
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

	return labelAddrs, nil
}

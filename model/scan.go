package model

import (
	"fmt"
	"strconv"
)

type ScanBaseResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
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

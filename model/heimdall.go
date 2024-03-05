package model

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/status-im/keycard-go/hexutils"

	"go-etl/client"
	"go-etl/config"
)

type Heimdall struct {
	Address       string     `json:"address"`
	FunctionCount int64      `json:"function_count"`
	MetaData      []MetaData `json:"metadatas"`
}
type MetaData struct {
	ControlStatements []string `json:"control_statements"`
	Selector          string   `json:"selector"`
	Payable           bool     `json:"payable"`
	View              bool     `json:"view"`
}

func (hdl *Heimdall) Get(address string, byteCode []byte) error {
	url := fmt.Sprintf("%s/decompile", config.Conf.ETL.HeimdallServer)
	body := map[string]string{
		"address":  address,
		"bytecode": hexutils.BytesToHex(byteCode),
	}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marhsal is data for heimdall is err %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("compose request for heimdall is err %v", err)
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("get response for heimdall is err %v", err)
	}

	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read data from res body is err %v", err)
	}
	if err = json.Unmarshal(b, hdl); err != nil {
		return fmt.Errorf("unmarshal the json data from resp body is err %v", err)
	}
	return nil
}

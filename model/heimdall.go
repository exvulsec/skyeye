package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/status-im/keycard-go/hexutils"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/config"
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
	ctx, _ := context.WithTimeout(context.TODO(), time.Second*5)
	req.WithContext(ctx)
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

type HeimdallPolicyCalc struct {
	Heimdall Heimdall
}

func (hdpc *HeimdallPolicyCalc) Calc(tx *SkyEyeTransaction) int {
	if hdpc.GetPolicy(tx) {
		return 30
	}
	return 0
}

func (hdpc *HeimdallPolicyCalc) Name() string {
	return "Heimdall"
}

func (hdpc *HeimdallPolicyCalc) GetPolicy(tx *SkyEyeTransaction) bool {
	for _, metadata := range hdpc.Heimdall.MetaData {
		if metadata.View {
			for _, statement := range metadata.ControlStatements {
				if statement == "if (msg.sender == (address(storage[0]))) { .. }" {
					return true
				}
			}
		}
	}
	return false
}

func (hdpc *HeimdallPolicyCalc) Filter(tx *SkyEyeTransaction) bool {
	hdl := Heimdall{}
	if err := hdl.Get(tx.ContractAddress, tx.ByteCode); err != nil {
		logrus.Error(err)
		return false
	}

	if hdl.FunctionCount > 10 {
		return true
	}

	hdpc.Heimdall = hdl
	return false
}

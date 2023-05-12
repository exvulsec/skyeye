package model

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"go-etl/client"
	"go-etl/datastore"
	"go-etl/utils"
)

type DeDaubRequest struct {
	HexByteCode string `json:"hex_bytecode"`
}

type DeDaubResponseString string

type DeDaubResponse struct {
	MD5          string `json:"md5"`
	ByteCode     string `json:"byteCode"`
	Disassembled string `json:"disassembled"`
	Source       string `json:"source"`
}

type DeDaub struct {
	Chain   string `gorm:"column:chain"`
	Address string `gorm:"column:address"`
	MD5     string `gorm:"column:md5"`
}

func (drs *DeDaubResponseString) GetCodeMD5(bytecode []byte) error {
	headers := map[string]string{
		"authority":          "api.dedaub.com",
		"accept":             "*/*",
		"accept-language":    "zh,zh-CN;q=0.9",
		"content-type":       "application/json",
		"origin":             "https://library.dedaub.com",
		"referer":            "https://library.dedaub.com/",
		"sec-ch-ua":          `"Chromium";v="112", "Google Chrome";v="112", "Not:A-Brand";v="99"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": "macOS",
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-site",
		"user-agent":         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36",
	}

	body, err := json.Marshal(DeDaubRequest{HexByteCode: hexutil.Encode(bytecode)})
	if err != nil {
		return fmt.Errorf("marhsall json from byte code is err: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.dedaub.com/api/on_demand/", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("new request for get meta dock labels is err: %v", err)
	}
	for key, value := range headers {
		req.Header.Add(key, value)
	}

	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("receive response from https://api.dedaub.com/api/on_demand is err: %v", err)
	}
	var reader io.Reader = resp.Body
	if utils.CheckHeaderIsGZip(resp.Header) {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("create gzip reader is err: %v", err)
		}
		defer gr.Close()
		reader = gr
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read data from resp.Body is err: %v", err)
	}
	defer resp.Body.Close()

	dataString := DeDaubResponseString(data)
	*drs = dataString
	return nil
}

func (dr *DeDaubResponse) GetSource(md5 string) error {
	headers := map[string]string{
		"authority":          "api.dedaub.com",
		"accept":             "*/*",
		"accept-language":    "zh,zh-CN;q=0.9",
		"content-type":       "application/json",
		"origin":             "https://library.dedaub.com",
		"referer":            "https://library.dedaub.com/",
		"sec-ch-ua":          `"Chromium";v="112", "Google Chrome";v="112", "Not:A-Brand";v="99"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": "macOS",
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-site",
		"user-agent":         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36",
	}
	url := fmt.Sprintf("https://api.dedaub.com/api/on_demand/decompilation/%s", md5)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("new request for get meta dock labels is err: %v", err)
	}
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("receive response from %s is err: %v", url, err)
	}
	var reader io.Reader = resp.Body
	if utils.CheckHeaderIsGZip(resp.Header) {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("create gzip reader is err: %v", err)
		}
		defer gr.Close()
		reader = gr
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read data from resp.Body is err: %v", err)
	}
	defer resp.Body.Close()
	if err = json.Unmarshal(data, dr); err != nil {
		return fmt.Errorf("unmarhsall dedaub data is err: %v", err)
	}
	return nil
}

func (d *DeDaub) Create() error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableDeDaub)
	return datastore.DB().Table(tableName).Create(d).Error
}

func (d *DeDaub) Get() error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableDeDaub)
	return datastore.DB().Table(tableName).
		Where("chain = ?", d.Chain).
		Where("address = ?", d.Address).
		Find(d).Error
}

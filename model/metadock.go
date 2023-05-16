package model

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go-etl/client"
	"go-etl/utils"
)

type MetaDockLabelRequest struct {
	Chain     string   `json:"chain"`
	Addresses []string `json:"addresses"`
}

type MetaDockLabelResponse struct {
	Address string `json:"address"`
	Label   string `json:"label"`
}

type MetaDockLabelsResponse []MetaDockLabelResponse

func (labels *MetaDockLabelsResponse) GetLabels(chain string, addrs []string) error {
	headers := map[string]string{
		"authority":          "extension.blocksec.com",
		"accept":             "application/json",
		"blocksec-meta-dock": "v2.4.0",
		"user-agent":         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36",
		"content-type":       "application/json",
		"origin":             "chrome-extension://fkhgpeojcbhimodmppkbbliepkpcgcoo",
		"sec-fetch-site":     "none",
		"sec-fetch-mode":     "cors",
		"sec-fetch-dest":     "empty",
		"accept-encoding":    "gzip, deflate, br",
		"accept-language":    "zh,zh-CN;q=0.9",
	}

	metaDockChain := utils.ConvertChainToMetaDock(chain)
	body, err := json.Marshal(MetaDockLabelRequest{
		Chain:     metaDockChain,
		Addresses: addrs,
	})
	if err != nil {
		return fmt.Errorf("marhsall json from chain %s and addrs %+v is err: %v", chain, addrs, err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://extension.blocksec.com/api/v1/address-label", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("new request for get meta dock labels is err: %v", err)
	}
	for key, value := range headers {
		req.Header.Add(key, value)
	}

	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("receive response from https://extension.blocksec.com/api/v1/address-label is err: %v", err)
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

	if err = json.Unmarshal(data, labels); err != nil {
		return fmt.Errorf("unmarhsall data from resp.Body %s, request is %s is err: %v", data, body, err)
	}
	return nil
}

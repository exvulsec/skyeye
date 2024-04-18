package model

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/client"
	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
	"github.com/exvulsec/skyeye/utils"
)

type OpenChainResponse struct {
	OK     bool            `json:"ok"`
	Result OpenChainResult `json:"result"`
}

type OpenChainResult struct {
	Function map[string][]OpenChainSignature `json:"function"`
}

type OpenChainSignature struct {
	Name     string `json:"name"`
	Filtered bool   `json:"filtered"`
}

type Signature struct {
	ByteSign string `json:"byte_sign" gorm:"column:byte_sign"`
	TextSign string `json:"text_sign" gorm:"column:text_sign"`
}

type Signatures []Signature

func (s *Signature) GetTextSign() error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableSignatures)
	return datastore.DB().Table(tableName).Where("byte_sign = ?", s.ByteSign).Limit(1).Find(s).Error
}

func (s *Signatures) Create() error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableSignatures)
	return datastore.DB().Table(tableName).CreateInBatches(s, config.Conf.Postgresql.MaxOpenConns).Error
}

func GetSignatures(byteSigns []string) ([]string, error) {
	textSignatures := []string{}
	httpByteSignatures := []string{}
	wg := sync.WaitGroup{}
	rwMutex := sync.RWMutex{}
	for _, byteSign := range byteSigns {
		wg.Add(1)
		go func(byteSign string) {
			defer func() {
				wg.Done()
			}()
			s := Signature{
				ByteSign: byteSign,
			}
			if err := s.GetTextSign(); err != nil {
				logrus.Errorf("get byte sign %s is err %v", byteSign, err)
				return
			}
			rwMutex.Lock()
			if s.TextSign != "" {
				textSignatures = append(textSignatures, s.TextSign)
			} else {
				httpByteSignatures = append(httpByteSignatures, s.ByteSign)
			}
			rwMutex.Unlock()
		}(byteSign)
	}
	wg.Wait()

	retry := 3
	signs := strings.Join(httpByteSignatures, ",")
	or := OpenChainResponse{}
	for {
		url := fmt.Sprintf("https://api.openchain.xyz/signature-database/v1/lookup?function=%s&filter=true", signs)
		resp, err := client.HTTPClient().Get(url)
		if err != nil {
			return []string{}, fmt.Errorf("receive response from %s is err: %v", url, err)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return []string{}, fmt.Errorf("read data from resp.Body is err: %v", err)
		}
		defer resp.Body.Close()
		if err = json.Unmarshal(data, &or); err != nil {
			return []string{}, fmt.Errorf("unmarshall data %s is err: %v", string(data), err)
		}
		if or.OK || retry == 0 {
			break
		}
		retry -= 1
	}
	if retry == 0 {
		return []string{}, fmt.Errorf("get signature from openchain retry 3 times is not ok")
	}
	byteSignatures := []string{}
	createSignatures := Signatures{}
	for _, byteSign := range httpByteSignatures {
		if value, ok := or.Result.Function[byteSign]; ok {
			if len(value) > 0 {
				textSignatures = append(textSignatures, value[0].Name)
				createSignatures = append(createSignatures, Signature{
					ByteSign: byteSign,
					TextSign: value[0].Name,
				})
			} else {
				byteSignatures = append(byteSignatures, byteSign)
			}
		}
	}

	sort.SliceStable(textSignatures, func(i, j int) bool {
		return textSignatures[i] < textSignatures[j]
	})

	if len(byteSignatures) > 0 {
		textSignatures = append(textSignatures, fmt.Sprintf("0x{%d}", len(byteSignatures)))
	}
	if err := createSignatures.Create(); err != nil {
		logrus.Errorf("insert %d signatures to db is err: %v", len(createSignatures), err)
	}
	return textSignatures, nil
}

package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"go-etl/datastore"
	"go-etl/utils"
)

type NastiffTransaction struct {
	Chain              string         `json:"chain"`
	BlockTimestamp     int64          `json:"block_timestamp" gorm:"column:block_timestamp"`
	BlockNumber        int64          `json:"block_number" gorm:"column:blknum"`
	TxHash             string         `json:"txhash" gorm:"column:txhash"`
	TxPos              int64          `json:"txpos" gorm:"column:txpos"`
	FromAddress        string         `json:"from_address" gorm:"column:from_address"`
	ContractAddress    string         `json:"contract_address" gorm:"column:contract_address"`
	Nonce              uint64         `json:"nonce" gorm:"column:nonce"`
	Policies           string         `json:"policies" gorm:"column:policies"`
	Score              int            `json:"score" gorm:"column:score"`
	NastiffValues      map[string]any `json:"nastiff_values" gorm:"-"`
	NastiffValuesBytes []byte         `json:"-" gorm:"column:nastiff_values"`
	ByteCode           []byte         `json:"-" gorm:"-"`
	Push4Args          []string       `json:"-" gorm:"-"`
	Push20Args         []string       `json:"-" gorm:"-"`
}

func (nt *NastiffTransaction) ConvertFromTransaction(tx Transaction) {
	nt.BlockTimestamp = tx.BlockTimestamp
	nt.BlockNumber = tx.BlockNumber
	nt.TxHash = tx.TxHash
	nt.TxPos = tx.TxPos
	nt.FromAddress = tx.FromAddress
	nt.ContractAddress = tx.ContractAddress
	nt.Nonce = tx.Nonce
}

func (nt *NastiffTransaction) ComposeNastiffValues(isNastiff bool, openAPIServer string) error {
	var err error
	codeSize := 0
	if len(nt.ByteCode) != 0 {
		codeSize = len(nt.ByteCode[2:])
	}

	values := map[string]any{
		"chain":      utils.ConvertChainToDeFiHackLabChain(nt.Chain),
		"txhash":     nt.TxHash,
		"createTime": time.Unix(nt.BlockTimestamp, 0).Format("2006-01-02 15:04:05"),
		"contract":   nt.ContractAddress,
		"func":       strings.Join(nt.Push4Args, ","),
		"push20":     strings.Join(nt.Push20Args, ","),
		"score":      nt.Score,
		"codeSize":   codeSize,
	}
	if isNastiff {
		var fund string
		scanTxResp, err := GetSourceEthAddress(nt.Chain, nt.ContractAddress, openAPIServer)
		if err != nil {
			logrus.Errorf("get contract %s's eth source is err: %v", nt.ContractAddress, err)
		}
		if scanTxResp.Address != "" {
			label := scanTxResp.Label
			if label == "" {
				if len(scanTxResp.Nonce) == 5 {
					label = "UnKnown"
				} else {
					label = scanTxResp.Address
				}
			}
			fund = fmt.Sprintf("%d-%s", len(scanTxResp.Nonce), label)
		} else {
			fund = "0-scanError"
		}
		values["fund"] = fund
	}

	nt.NastiffValuesBytes, err = json.Marshal(values)
	nt.NastiffValues = values
	if err != nil {
		return fmt.Errorf("marhsal nastiffValues is err %v", err)
	}
	return nil
}

func (nt *NastiffTransaction) Insert(isNastiff bool, openAPIServer string) error {
	if len(nt.NastiffValues) == 0 {
		if err := nt.ComposeNastiffValues(isNastiff, openAPIServer); err != nil {
			return err
		}
	}
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableNastiffTransaction)
	return datastore.DB().Table(tableName).Create(nt).Error
}

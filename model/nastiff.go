package model

import (
	"encoding/json"
	"fmt"
	"strings"

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
	NastiffValues      map[string]any `json:"nastiff_values" gorm:"-"`
	NastiffValuesBytes []byte         `json:"-" gorm:"column:nastiff_values"`
	ByteCode           []byte         `json:"-" gorm:"-"`
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

	opCodeArgs := GetPushTypeArgs(nt.ByteCode)

	values := map[string]any{
		"chain":    utils.ConvertChainToDeFiHackLabChain(nt.Chain),
		"txhash":   nt.TxHash,
		"contract": nt.ContractAddress,
		"func":     strings.Join(GetPush4Args(opCodeArgs[utils.PUSH4]), ","),
		"push20":   strings.Join(GetPush20Args(nt.Chain, opCodeArgs[utils.PUSH20]), ","),
		"codeSize": codeSize,
	}
	if isNastiff {
		scanTxResp, err := GetSourceEthAddress(nt.Chain, nt.ContractAddress, openAPIServer)
		if err != nil {
			return fmt.Errorf("get contract %s's eth source is err: %v", nt.ContractAddress, err)
		}
		fund := scanTxResp.Address
		if scanTxResp.Address != "" {
			if len(scanTxResp.Nonce) == 5 {
				label := scanTxResp.Label
				if label == "" {
					label = "UnKnown"
				}
				fund = fmt.Sprintf("%d-%s", len(scanTxResp.Nonce), label)
			}
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

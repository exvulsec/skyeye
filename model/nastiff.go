package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-etl/datastore"
	"go-etl/utils"
)

type NastiffTransaction struct {
	Chain              string            `json:"chain"`
	BlockTimestamp     int64             `json:"block_timestamp" gorm:"column:block_timestamp"`
	BlockNumber        int64             `json:"block_number" gorm:"column:blknum"`
	TxHash             string            `json:"txhash" gorm:"column:txhash"`
	TxPos              int64             `json:"txpos" gorm:"column:txpos"`
	FromAddress        string            `json:"from_address" gorm:"column:from_address"`
	ContractAddress    string            `json:"contract_address" gorm:"column:contract_address"`
	Nonce              uint64            `json:"nonce" gorm:"column:nonce"`
	Score              int               `json:"score" gorm:"column:score"`
	SplitScores        string            `json:"split_scores" gorm:"column:split_scores"`
	NastiffValues      map[string]string `json:"nastiff_values" gorm:"-"`
	NastiffValuesBytes []byte            `json:"-" gorm:"column:nastiff_values"`
	ByteCode           []byte            `json:"-" gorm:"-"`
	Push4Args          []string          `json:"-" gorm:"-"`
	Push20Args         []string          `json:"-" gorm:"-"`
	Fund               string            `json:"-" gorm:"-"`
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

func (nt *NastiffTransaction) hasFlashLoan(flashLoanFuncNames []string) bool {
	for _, push4 := range nt.Push4Args {
		for _, funcName := range flashLoanFuncNames {
			if push4 == funcName {
				return true
			}
		}
	}
	return false
}

func (nt *NastiffTransaction) ComposeNastiffValues() error {
	var err error
	codeSize := 0
	if len(nt.ByteCode) != 0 {
		codeSize = len(nt.ByteCode[2:])
	}

	values := map[string]string{
		"chain":        utils.ConvertChainToDeFiHackLabChain(nt.Chain),
		"txhash":       nt.TxHash,
		"createTime":   time.Unix(nt.BlockTimestamp, 0).Format("2006-01-02 15:04:05"),
		"contract":     nt.ContractAddress,
		"func":         strings.Join(nt.Push4Args, ","),
		"push20":       strings.Join(nt.Push20Args, ","),
		"codeSize":     fmt.Sprintf("%d", codeSize),
		"fund":         nt.Fund,
		"score":        fmt.Sprintf("%d", nt.Score),
		"split_scores": nt.SplitScores,
	}

	nt.NastiffValuesBytes, err = json.Marshal(values)
	if err != nil {
		return fmt.Errorf("marhsal nastiffValues is err %v", err)
	}
	nt.NastiffValues = values
	return nil
}

func (nt *NastiffTransaction) Insert() error {
	if len(nt.NastiffValues) == 0 {
		if err := nt.ComposeNastiffValues(); err != nil {
			return err
		}
	}
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableNastiffTransaction)
	return datastore.DB().Table(tableName).Create(nt).Error
}

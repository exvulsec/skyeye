package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-etl/datastore"
	"go-etl/utils"
)

type SkyEyeTransaction struct {
	Chain           string   `json:"chain"`
	BlockTimestamp  int64    `json:"block_timestamp" gorm:"column:block_timestamp"`
	BlockNumber     int64    `json:"block_number" gorm:"column:blknum"`
	TxHash          string   `json:"txhash" gorm:"column:txhash"`
	TxPos           int64    `json:"txpos" gorm:"column:txpos"`
	FromAddress     string   `json:"from_address" gorm:"column:from_address"`
	ContractAddress string   `json:"contract_address" gorm:"column:contract_address"`
	MultiContract   []string `json:"multi_contract" gorm:"-"`
	Nonce           uint64   `json:"nonce" gorm:"column:nonce"`
	Score           int      `json:"score" gorm:"column:score"`
	SplitScores     string   `json:"split_scores" gorm:"column:split_scores"`
	Values          []byte   `json:"-" gorm:"column:nastiff_values"`
	ByteCode        []byte   `json:"-" gorm:"-"`
	Push4Args       []string `json:"-" gorm:"-"`
	Push20Args      []string `json:"-" gorm:"-"`
	Push32Args      []string `json:"-" gorm:"-"`
	PushStringLogs  []string `json:"-" gorm:"-"`
	Fund            string   `json:"-" gorm:"-"`
	IsMultiContract bool     `json:"-" gorm:"-"`
}

func (st *SkyEyeTransaction) ConvertFromTransaction(tx Transaction) {
	multiContracts := strings.Split(tx.ContractAddress, ",")
	st.BlockTimestamp = tx.BlockTimestamp
	st.BlockNumber = tx.BlockNumber
	st.TxHash = tx.TxHash
	st.TxPos = tx.TxPos
	st.FromAddress = tx.FromAddress
	st.ContractAddress = multiContracts[0]
	st.Nonce = tx.Nonce
}

func (st *SkyEyeTransaction) hasFlashLoan(flashLoanFuncNames []string) bool {
	for _, push4 := range st.Push4Args {
		for _, funcName := range flashLoanFuncNames {
			if push4 == funcName {
				return true
			}
		}
	}
	return false
}

func (st *SkyEyeTransaction) hasStart() bool {
	for _, push4 := range st.Push4Args {
		if strings.Contains(strings.ToLower(push4), "start") {
			return true
		}
	}
	return false
}

func (st *SkyEyeTransaction) hasRiskAddress(addrs []string) bool {
	for _, push20 := range st.Push20Args {
		for _, addr := range addrs {
			if push20 == addr {
				return true
			}
		}
	}
	return false
}

func (st *SkyEyeTransaction) ComposeSkyEyeTXValues() map[string]string {
	values := map[string]string{
		"Chain":      utils.ConvertChainToDeFiHackLabChain(st.Chain),
		"Block":      fmt.Sprintf("%d", st.BlockNumber),
		"TXhash":     st.TxHash,
		"CreateTime": fmt.Sprintf("%s UTC", time.Unix(st.BlockTimestamp, 0).Format(time.DateTime)),
		"Contract":   st.ContractAddress,
		"Deployer":   st.FromAddress,
		"Fund":       st.Fund,
	}

	keys := []string{"Func", "AddrLabels", "CodeSize", "Score", "SplitScores", "EmitLogs"}
	byteCodeValues := st.ComposeSkyEyeTXValuesFromByteCode()
	for _, key := range keys {
		value := byteCodeValues[key]
		values[key] = value
	}
	return values
}

func (st *SkyEyeTransaction) ComposeSkyEyeTXValuesFromByteCode() map[string]string {
	codeSize := 0
	if len(st.ByteCode) != 0 {
		codeSize = len(st.ByteCode[2:])
	}

	return map[string]string{
		"Func":        strings.Join(st.Push4Args, ","),
		"AddrLabels":  strings.Join(st.Push20Args, ","),
		"CodeSize":    fmt.Sprintf("%d", codeSize),
		"Score":       fmt.Sprintf("%d", st.Score),
		"SplitScores": st.SplitScores,
		"EmitLogs":    strings.Join(st.PushStringLogs, ","),
	}
}

func (st *SkyEyeTransaction) Insert() error {
	var err error
	if len(st.Values) == 0 {
		st.Values, err = json.Marshal(st.ComposeSkyEyeTXValues())
		if err != nil {
			return fmt.Errorf("marhsal nastiffValues is err %v", err)
		}
	}
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableNastiffTransaction)
	return datastore.DB().Table(tableName).Create(st).Error
}

func (st *SkyEyeTransaction) GetLatestRecord(chain string) error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableNastiffTransaction)
	return datastore.DB().Table(tableName).Where("chain = ?", chain).Order("created_at DESC").Limit(1).Find(st).Error
}

func (st *SkyEyeTransaction) GetInfoByContract(chain, contract string) error {
	tableName := utils.ComposeTableName(datastore.SchemaPublic, datastore.TableNastiffTransaction)
	return datastore.DB().Table(tableName).Where("chain = ? AND contract_address = ?", chain, contract).Find(st).Error
}

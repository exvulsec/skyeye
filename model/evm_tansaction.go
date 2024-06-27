package model

import (
	"fmt"

	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/datastore"
)

var (
	FromAddressSource = "from_address"
	ToAddressSource   = "to_address"
)

type EVMTransaction struct {
	BlockTimestamp int64           `json:"block_timestamp" gorm:"column:block_timestamp"`
	BlockNumber    int64           `json:"block_number" gorm:"column:blknum"`
	TxHash         string          `json:"txhash" gorm:"column:txhash"`
	TxPos          int64           `json:"txpos" gorm:"column:txpos"`
	FromAddress    string          `json:"from_address" gorm:"column:from_address"`
	ToAddress      *string         `json:"to_address" gorm:"column:to_address"`
	TxType         uint8           `json:"tx_type" gorm:"column:tx_type"`
	Value          decimal.Decimal `json:"value" gorm:"column:value"`
	TxStatus       int64           `json:"tx_status" gorm:"column:tx_status"`
	Nonce          uint64          `json:"nonce" gorm:"column:nonce"`
}

func (et *EVMTransaction) TableName() string {
	return fmt.Sprintf("%s.%s", config.Conf.ETL.Chain, datastore.TableTransactions)
}

func (et *EVMTransaction) Create() error {
	return datastore.DB().Table(et.TableName()).Create(et).Error
}

func (et *EVMTransaction) ComposeNodeEdge(chain string) (NodeEdge, error) {
	token := Token{}
	if err := token.IsExisted(chain, EVMPlatformCurrency); err != nil {
		return NodeEdge{}, fmt.Errorf("get token %s on chain is err: %v", EVMPlatformCurrency, err)
	}

	return NodeEdge{
		Timestamp:   et.BlockTimestamp,
		TxHash:      et.TxHash,
		Value:       token.GetValueWithDecimalsAndSymbol(et.Value),
		FromAddress: et.FromAddress,
		ToAddress:   *et.ToAddress,
	}, nil
}

type EVMTransactions []EVMTransaction

func (ets *EVMTransactions) TableName(chain string) string {
	if chain == "" {
		chain = config.Conf.ETL.Chain
	}

	return fmt.Sprintf("%s.%s", chain, datastore.TableTransactions)
}

func (ets *EVMTransactions) GetByAddress(source, chain, address string) error {
	engine := datastore.DB().Table(ets.TableName(chain))
	switch source {
	case FromAddressSource:
		engine = engine.Where("from_address = ?", address)
	case ToAddressSource:
		engine = engine.Where("to_address = ?", address)
	}
	return engine.Find(ets).Error
}

func (ets *EVMTransactions) ComposeNodeEdges(chain string) []NodeEdge {
	nodeEdges := []NodeEdge{}
	for _, evmTX := range *ets {
		nodeEdge, err := evmTX.ComposeNodeEdge(chain)
		if err != nil {
			logrus.Error(err)
			continue
		}
		nodeEdges = append(nodeEdges, nodeEdge)
	}
	return nodeEdges
}

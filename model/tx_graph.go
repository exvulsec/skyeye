package model

import (
	"fmt"
	"sort"
	"strconv"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

type Node struct {
	Address string `json:"address"`
	Label   string `json:"label"`
}

type NodeEdge struct {
	TxHash        string          `json:"tx_hash,omitempty"`
	Timestamp     int64           `json:"timestamp,omitempty"`
	FromAddress   string          `json:"from_address,omitempty"`
	ToAddress     string          `json:"to_address,omitempty"`
	Token         string          `json:"token,omitempty"`
	Value         decimal.Decimal `json:"-"`
	ValueWithUnit string          `json:"value,omitempty"`
	Index         int             `json:"index"`
}

type NodeEdges []NodeEdge

type Graph struct {
	Nodes []Node    `json:"nodes"`
	Edges NodeEdges `json:"edges"`
}

func NewGraphFromAssetTransfers(chain string, tx Transaction, assetTransfers AssetTransfers) (*Graph, error) {
	g := Graph{}
	if err := g.ConvertEdgeFromAssetTransfers(tx, assetTransfers); err != nil {
		return nil, err
	}
	g.Edges.Distinct()
	g.Edges.SetValueWithUnit(chain)
	g.AddNodes(chain)
	return &g, nil
}

func NewGraphFromScan(chain string, transactions []ScanTransaction) *Graph {
	g := Graph{}
	g.ConvertEdgeFromScanTransactions(chain, transactions)
	g.Edges.Distinct()
	g.Edges.SetValueWithUnit(chain)
	g.AddNodes(chain)
	return &g
}

func NewGraph(chain, address string) (*Graph, error) {
	g := Graph{}
	if err := g.AddNodeEdges(chain, address); err != nil {
		return nil, err
	}
	g.Edges.Distinct()
	g.Edges.SetValueWithUnit(chain)
	g.AddNodes(chain)
	return &g, nil
}

func (g *Graph) AddNodes(chain string) {
	if g.Edges == nil {
		return
	}
	addrs := []string{}
	for _, edge := range g.Edges {
		if edge.FromAddress != "" {
			addrs = append(addrs, edge.FromAddress)
		}
		if edge.ToAddress != "" {
			addrs = append(addrs, edge.ToAddress)
		}
	}
	distinctAddrs := mapset.NewSet[string](addrs...).ToSlice()
	sort.SliceStable(distinctAddrs, func(i, j int) bool {
		return distinctAddrs[i] < distinctAddrs[j]
	})

	for _, addr := range distinctAddrs {
		label := AddressLabel{Address: addr}
		if err := label.GetLabel(chain, addr); err != nil {
			logrus.Errorf("get address %s's label is err: %v", addr, err)
		}
		g.Nodes = append(g.Nodes, Node{Address: addr, Label: label.Label})
	}
}

func (g *Graph) ConvertEdgeFromScanTransactions(chain string, transactions []ScanTransaction) {
	if g.Edges == nil {
		g.Edges = NodeEdges{}
	}
	for _, transaction := range transactions {
		if err := transaction.ConvertStringToInt(); err != nil {
			logrus.Error(err)
			continue
		}
		toAddress := transaction.ToAddress
		if toAddress == "" {
			toAddress = transaction.Contract
		}
		decimals, err := strconv.ParseInt(transaction.TokenDecimals, 10, 64)
		if err != nil {
			logrus.Errorf("parse decimals %s to int64 err: %v", transaction.TokenDecimals, err)
		}
		t := Token{
			Address:  transaction.Contract,
			Name:     transaction.TokenName,
			Symbol:   transaction.TokenSymbol,
			Decimals: decimals,
		}

		if err := t.IsExisted(chain, transaction.Contract); err != nil {
			logrus.Errorf("get token info from db is err: %v", err)
		} else {
			if t.ID == nil {
				if createErr := t.Create(chain); createErr != nil {
					logrus.Errorf("create token info from db is err: %v", err)
				}
			}
		}
		value := transaction.Value
		if transaction.TokenID != nil {
			value = *transaction.TokenID
		}
		g.Edges = append(g.Edges, NodeEdge{
			FromAddress: transaction.FromAddress,
			ToAddress:   toAddress,
			Token:       transaction.Contract,
			Value:       value,
			TxHash:      transaction.TransactionHash,
			Timestamp:   transaction.Timestamp,
		})
	}
	sort.SliceStable(g.Edges, func(i, j int) bool {
		return g.Edges[i].Timestamp < g.Edges[j].Timestamp
	})
}

func (g *Graph) AddressFocus(address string, addrs []string) bool {
	for _, addr := range addrs {
		if address == addr {
			return true
		}
	}
	return false
}

func (g *Graph) ConvertEdgeFromAssetTransfers(tx Transaction, assetTransfers AssetTransfers) error {
	if g.Edges == nil {
		g.Edges = NodeEdges{}
	}

	for _, assetTransfer := range assetTransfers {
		g.Edges = append(g.Edges, NodeEdge{
			FromAddress: assetTransfer.From,
			ToAddress:   assetTransfer.To,
			Token:       assetTransfer.Address,
			Value:       assetTransfer.Value,
			TxHash:      tx.TxHash,
			Timestamp:   tx.BlockTimestamp,
		})
	}
	sort.SliceStable(g.Edges, func(i, j int) bool {
		return g.Edges[i].Timestamp > g.Edges[j].Timestamp
	})
	return nil
}

func (g *Graph) AddNodeEdges(chain, address string) error {
	if g.Edges == nil {
		g.Edges = NodeEdges{}
	}
	if err := g.Edges.ComposeNode(FromAddressSource, chain, address); err != nil {
		return err
	}
	if err := g.Edges.ComposeNode(ToAddressSource, chain, address); err != nil {
		return err
	}
	return nil
}

func (nes *NodeEdges) ComposeNode(source, chain, address string) error {
	txs := EVMTransactions{}
	xfers := TokenTransfers{}
	if err := txs.GetByAddress(source, chain, address); err != nil {
		return fmt.Errorf("get %s txs %s is from db is err: %v", source, address, err)
	}
	*nes = append(*nes, txs.ComposeNodeEdges()...)
	if err := xfers.GetByAddress(source, chain, address); err != nil {
		return fmt.Errorf("get %s xfers %s is from db is err: %v", source, address, err)
	}
	*nes = append(*nes, xfers.ComposeNodes()...)
	return nil
}

func (nes *NodeEdges) Distinct() {
	distinctNodeEdges := NodeEdges{}
	for _, ne := range *nes {
		ok, index := distinctNodeEdges.isSameSource(ne)
		if ok {
			distinctNodeEdges[index].Value = distinctNodeEdges[index].Value.Add(ne.Value)
		} else {
			ne.Index = len(distinctNodeEdges) + 1
			distinctNodeEdges = append(distinctNodeEdges, ne)
		}
	}
	*nes = distinctNodeEdges
}

func (nes *NodeEdges) isSameSource(ne NodeEdge) (bool, int) {
	for index := range *nes {
		if ne.FromAddress == (*nes)[index].FromAddress && ne.ToAddress == (*nes)[index].ToAddress && ne.Token == (*nes)[index].Token {
			return true, index
		}
	}
	return false, -1
}

func (nes *NodeEdges) SetValueWithUnit(chain string) {
	tokenMaps := map[string]Token{}
	var (
		token Token
		ok    bool
	)
	for index, edge := range *nes {
		if token, ok = tokenMaps[edge.Token]; !ok {
			if err := token.IsExisted(chain, edge.Token); err != nil {
				logrus.Errorf(fmt.Sprintf("get token %s is err %v", edge.Token, err))
			}
			if token.ID == nil {
				if err := token.GetMetadataOnChain(chain, edge.Token); err != nil {
					logrus.Error(err)
					token.Symbol = edge.Token
				}
			}
			tokenMaps[edge.Token] = token
		}
		token.Value = token.GetValueWithDecimals(edge.Value)
		edge.ValueWithUnit = fmt.Sprintf("%s %s", token.Value, token.Symbol)
		(*nes)[index] = edge
	}
}

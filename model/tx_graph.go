package model

import (
	"fmt"
	"sort"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

type NodeEdge struct {
	TxHash      string `json:"tx_hash,omitempty"`
	Timestamp   int64  `json:"timestamp,omitempty"`
	FromAddress string `json:"from_address,omitempty"`
	ToAddress   string `json:"to_address,omitempty"`
	Value       string `json:"value,omitempty"`
}

type NodeEdges []NodeEdge

type Graph struct {
	Nodes []string  `json:"nodes"`
	Edges NodeEdges `json:"edges"`
}

func NewGraphFromAssetTransfers(chain, txhash string, txTimestamp int64, assetTransfers AssetTransfers) (*Graph, error) {
	g := Graph{}
	if err := g.ConvertEdgeFromAssetTransfers(chain, txhash, txTimestamp, assetTransfers); err != nil {
		return nil, err
	}
	g.AddNodes()
	return &g, nil
}

func NewGraphFromScan(transactions []ScanTransaction) *Graph {
	g := Graph{}
	g.ConvertEdgeFromScanTransactions(transactions)
	g.AddNodes()
	return &g
}

func NewGraph(chain, address string) (*Graph, error) {
	g := Graph{}
	if err := g.AddNodeEdges(chain, address); err != nil {
		return nil, err
	}
	g.AddNodes()
	return &g, nil
}

func (g *Graph) AddNodes() {
	if g.Edges == nil {
		return
	}
	nodes := []string{}
	for _, edge := range g.Edges {
		if edge.FromAddress != "" {
			nodes = append(nodes, edge.FromAddress)
		}
		if edge.ToAddress != "" {
			nodes = append(nodes, edge.ToAddress)
		}
	}
	g.Nodes = mapset.NewSet[string](nodes...).ToSlice()
	sort.SliceStable(g.Nodes, func(i, j int) bool {
		return g.Nodes[i] < g.Nodes[j]
	})
}

func (g *Graph) ConvertEdgeFromScanTransactions(transactions []ScanTransaction) {
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
		value := fmt.Sprintf("%s %s", transaction.Value.DivRound(decimal.NewFromInt32(10).Pow(transaction.TokenDecimal), 6), transaction.TokenSymbol)
		g.Edges = append(g.Edges, NodeEdge{
			FromAddress: transaction.FromAddress,
			ToAddress:   toAddress,
			Value:       value,
			TxHash:      transaction.TransactionHash,
			Timestamp:   transaction.Timestamp,
		})
	}
	sort.SliceStable(g.Edges, func(i, j int) bool {
		return g.Edges[i].Timestamp > g.Edges[j].Timestamp
	})
}

func (g *Graph) ConvertEdgeFromAssetTransfers(chain, txHash string, txTimestamp int64, assetTransfers AssetTransfers) error {
	if g.Edges == nil {
		g.Edges = NodeEdges{}
	}
	tokenMaps := map[string]Token{}

	for _, assetTransfer := range assetTransfers {
		if assetTransfer.Value.Equal(decimal.Decimal{}) {
			continue
		}
		var (
			token Token
			ok    bool
		)
		if token, ok = tokenMaps[assetTransfer.Address]; !ok {
			if err := token.IsExisted(chain, assetTransfer.Address); err != nil {
				return fmt.Errorf("get token %s is err %v", assetTransfer.Address, err)
			}
			tokenMaps[assetTransfer.Address] = token
		}

		if token.ID == nil {
			token.Symbol = token.Address
		}
		token.Value = token.GetValueWithDecimals(assetTransfer.Value)
		valueWithUnit := fmt.Sprintf("%s %s", token.Value, token.Symbol)

		g.Edges = append(g.Edges, NodeEdge{
			FromAddress: assetTransfer.From,
			ToAddress:   assetTransfer.To,
			Value:       valueWithUnit,
			TxHash:      txHash,
			Timestamp:   txTimestamp,
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

func (ns *NodeEdges) ComposeNode(source, chain, address string) error {
	txs := EVMTransactions{}
	xfers := TokenTransfers{}
	if err := txs.GetByAddress(source, chain, address); err != nil {
		return fmt.Errorf("get %s txs %s is from db is err: %v", source, address, err)
	}
	*ns = append(*ns, txs.ComposeNodeEdges(chain)...)
	if err := xfers.GetByAddress(source, chain, address); err != nil {
		return fmt.Errorf("get %s xfers %s is from db is err: %v", source, address, err)
	}
	*ns = append(*ns, xfers.ComposeNodes(chain)...)
	return nil
}

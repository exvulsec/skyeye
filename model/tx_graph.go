package model

import (
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
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
		nodes = append(nodes, edge.FromAddress, edge.ToAddress)
	}
	g.Nodes = mapset.NewSet[string](nodes...).ToSlice()
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
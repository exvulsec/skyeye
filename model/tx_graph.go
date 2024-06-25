package model

import (
	"fmt"
	"math"
	"time"

	"github.com/fogleman/gg"
	"github.com/shopspring/decimal"
)

const (
	W           = 2560
	H           = 1440
	SplitHeight = 200
	SplitWight  = 800
	RectW       = 350
	RectH       = 120
)

type Coordinate struct {
	X, Y float64
}

type Node struct {
	Address    string          `json:"address"`
	TxHash     string          `json:"tx_hash"`
	Timestamp  int64           `json:"timestamp"`
	Token      string          `json:"token"`
	Value      decimal.Decimal `json:"value"`
	Coordinate Coordinate      `json:"-"`
	FromNodes  []Node
	ToNodes    []Node
}

func (n *Node) ComposeWithSource(source, chain, address string) ([]Node, error) {
	txs := EVMTransactions{}
	xfers := TokenTransfers{}
	nodes := []Node{}
	if err := txs.GetByAddress(source, chain, address); err != nil {
		return nil, fmt.Errorf("get %s txs %s is from db is err: %v", source, address, err)
	}
	nodes = append(nodes, txs.ComposeNodes(source)...)
	if err := xfers.GetByAddress(source, chain, address); err != nil {
		return nil, fmt.Errorf("get %s xfers %s is from db is err: %v", source, address, err)
	}
	nodes = append(nodes, xfers.ComposeNodes(source)...)
	return nodes, nil
}

func (n *Node) Compose(chain, address string) error {
	n.Address = address
	toNodes, err := n.ComposeWithSource(FromAddressSource, chain, address)
	if err != nil {
		return err
	}
	n.ToNodes = toNodes
	fromNodes, err := n.ComposeWithSource(ToAddressSource, chain, address)
	if err != nil {
		return err
	}
	n.FromNodes = fromNodes
	return nil
}

func DrawGraph(chain string, n Node) {
	dc := gg.NewContext(W, H)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	n.DrawRectangle(dc, W/2, H/2, "#E8D673")
	n.DrawAssociateNodes(dc, n.FromNodes, chain, FromAddressSource)
	n.DrawAssociateNodes(dc, n.ToNodes, chain, ToAddressSource)
	dc.SavePNG("graph.png")
}

func (n *Node) DrawAssociateNodes(dc *gg.Context, nodes []Node, chain, source string) {
	x := n.Coordinate.X
	y := n.Coordinate.Y + RectH/2
	switch source {
	case FromAddressSource:
		x = x - SplitWight
	case ToAddressSource:
		x = x + SplitWight + RectW
	}
	midCount := len(nodes) / 2
	dy := 0.0
	for index, node := range nodes {
		if index < len(nodes)/2 {
			dy = y - (SplitHeight+RectH)*(float64(midCount-index))
		} else if index >= midCount {
			if index == midCount {
				if len(nodes)%2 != 0 {
					dy = y
				} else {
					dy = y + (SplitHeight+RectH)*(float64(index-midCount)+1)
				}
			} else {
				dy = y + (SplitHeight+RectH)*(float64(index-midCount))
			}
		}

		node.DrawRectangle(dc, x, dy, "#BDCCBD")

		lineX1, lineY1 := 0.0, 0.0
		lineX2, lineY2 := 0.0, 0.0
		switch source {
		case FromAddressSource:
			lineX1, lineY1 = x+RectW/2, dy
			lineX2 = n.Coordinate.X
			if index < len(nodes)/2 {
				lineY2 = y - 10*(float64(midCount-index))
			} else if index >= midCount {
				if index == midCount {
					if len(nodes)%2 != 0 {
						lineY2 = y
					} else {
						lineY2 = y + 10*(float64(index-midCount)+1)
					}
				} else {
					lineY2 = y + 10*(float64(index-midCount))
				}
			}

		case ToAddressSource:
			lineX1, lineY1 = n.Coordinate.X+RectW, n.Coordinate.Y+RectH/2
			lineX2, lineY2 = x-RectW/2, dy
		}
		tokenSymbol := "Ether"
		if node.Token != "" {
			token := Token{}
			token.IsExisted(chain, node.Token)
			tokenSymbol = token.Symbol
		}
		text := fmt.Sprintf("%s %s %s %s", time.Unix(node.Timestamp, 0).Format(time.DateTime), node.TxHash, node.Value, tokenSymbol)
		node.DrawLineWithArrow(dc, lineX1, lineY1, lineX2, lineY2, text)
		nodes[index] = node
	}
}

func (n *Node) DrawRectangle(dc *gg.Context, x, y float64, hexColor string) {
	rectX, rectY := x-RectW/2, y-RectH/2
	dc.SetHexColor(hexColor)
	dc.DrawRoundedRectangle(rectX, rectY, RectW, RectH, 30)
	dc.Fill()
	dc.Stroke()
	DrawString(dc, x, y, "", n.Address)
	n.Coordinate.X, n.Coordinate.Y = rectX, rectY
}

func DrawString(dc *gg.Context, x, y float64, hexColor, text string) {
	if hexColor == "" {
		hexColor = "#000000"
	}
	dc.SetHexColor(hexColor)
	dc.LoadFontFace("/Users/muzry/Library/Fonts/0xProtoNerdFontMono-Regular.ttf", 12)
	textWidth, textHeight := dc.MeasureString(text)
	dc.DrawString(text, x-textWidth/2, y+textHeight/2)
	dc.Stroke()
}

func (n *Node) DrawLineWithArrow(dc *gg.Context, x1, y1, x2, y2 float64, text string) {
	dc.SetHexColor("#000000")
	dc.SetLineWidth(4)
	dc.DrawLine(x1, y1, x2, y2)
	dc.Stroke()
	n.DrawArrow(dc, x1, y1, x2, y2)
	if text != "" {
		angle := math.Atan2(y2-y1, x2-x1)
		// 获取字符串的尺寸
		textWidth := 250.0 // 设置文本框的宽度
		textHeight := dc.FontHeight()

		// 计算字符串的中心位置，使其位于线条的中间上方
		midX := (x1 + x2) / 2
		midY := (y1 + y2) / 2
		offset := 30.0 // 调整文本距离线条的偏移量
		textX := midX - textWidth/2 + 120
		textY := midY - textHeight/2 - offset

		// 平移和旋转上下文
		dc.Push()
		dc.RotateAbout(angle, midX, midY)
		dc.LoadFontFace("/Users/muzry/Library/Fonts/0xProtoNerdFontMono-Regular.ttf", 10)
		dc.DrawStringWrapped(text, textX, textY, 0.5, 0.5, textWidth, 1.5, gg.AlignCenter)
		dc.Pop()
	}
}

func (n *Node) DrawArrow(dc *gg.Context, x1, y1, x2, y2 float64) {
	arrowLength := 10.0
	arrowAngle := math.Pi / 6.0

	angle := math.Atan2(y2-y1, x2-x1)

	x3 := x2 - arrowLength*math.Cos(angle-arrowAngle)
	y3 := y2 - arrowLength*math.Sin(angle-arrowAngle)
	x4 := x2 - arrowLength*math.Cos(angle+arrowAngle)
	y4 := y2 - arrowLength*math.Sin(angle+arrowAngle)

	dc.DrawLine(x2, y2, x3, y3)
	dc.DrawLine(x2, y2, x4, y4)
	dc.Stroke()
}

package notifier

import (
	"fmt"

	"github.com/go-lark/lark"
	"github.com/go-lark/lark/card"
	"github.com/sirupsen/logrus"
)

type larkNotifier struct {
	larkBot *lark.Bot
}

type LarkCard struct {
	Title      string
	ColumnSets []LarkColumnSet
	Actions    []LarkAction
}

type LarkColumnSet struct {
	Name    string
	Columns []LarkColumn
}

type LarkColumn struct {
	Name   string
	Value  string
	Weight int
}

type LarkAction struct {
	Name string
	URL  string
}

func NewLarkNotifier(webHookURL string) Notifier {
	return &larkNotifier{larkBot: lark.NewNotificationBot(webHookURL)}
}

func (ln *larkNotifier) Name() string {
	return LarkNotifierName
}

func (ln *larkNotifier) Notify(larkData any) {
	outMsg := ln.GetOutComingMsg(larkData)
	_, err := ln.larkBot.PostNotificationV2(outMsg)
	if err != nil {
		logrus.Errorf("send message to lark is err: %v", err)
		return
	}
}

func (ln *larkNotifier) composeCardOutComingMsg(data LarkCard) lark.OutcomingMessage {
	msg := lark.NewMsgBuffer(lark.MsgInteractive)
	cardString := ln.ComposeCard(data).String()
	return msg.Card(cardString).Build()
}

func (ln *larkNotifier) GetOutComingMsg(larkData any) lark.OutcomingMessage {
	switch data := larkData.(type) {
	case LarkCard:
		return ln.composeCardOutComingMsg(data)
	}
	return lark.OutcomingMessage{}
}

func (ln *larkNotifier) ComposeCard(data LarkCard) *card.Block {
	builder := lark.NewCardBuilder()
	elements := []card.Element{}
	for _, set := range data.ColumnSets {
		if set.Name == "HR" {
			elements = append(elements, builder.Hr())
		} else {
			elements = append(elements, ln.ComposeColumnSet(builder, set.Columns))
		}
	}
	elements = append(elements, builder.Hr())
	for _, action := range data.Actions {
		elements = append(elements, ln.ComposeAction(builder, action))
	}

	return builder.Card(elements...).Title(data.Title).Red()
}

func (ln *larkNotifier) ComposeColumnSet(builder *lark.CardBuilder, larkColumns []LarkColumn) *card.ColumnSetBlock {
	columns := []*card.ColumnBlock{}
	for _, column := range larkColumns {
		columns = append(columns, ln.ComposeColumn(builder, column.Name, column.Value, column.Weight))
	}

	return builder.ColumnSet(columns...).
		FlexMode("bisect").
		HorizontalSpacing("default")
}

func (ln *larkNotifier) ComposeColumn(builder *lark.CardBuilder, key string, value any, weight any) *card.ColumnBlock {
	weightInt := weight.(int)
	text := builder.Text(fmt.Sprintf("**%s:**\n%s", key, value)).LarkMd()

	return builder.Column(
		builder.Div().Text(text)).
		VerticalAlign("top").
		Width("weighted").
		Weight(weightInt)
}

func (ln *larkNotifier) ComposeAction(builder *lark.CardBuilder, action LarkAction) *card.ActionBlock {
	return builder.Action(builder.Button(card.Text(action.Name)).Primary().URL(action.URL))
}

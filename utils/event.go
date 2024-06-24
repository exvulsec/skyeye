package utils

import (
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
)

var TopicMaps = map[string]string{
	"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef": "Transfer",
	"0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65": "Withdrawal",
	"0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c": "Deposit",
	"0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925": "Approval",
	"0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31": "ApprovalForAll",
	"0xda9fa7c1b00402c17d0161b249b1ab8bbec047c5a52207b9c112deffd817036b": "Permit2",
}

const (
	TransferTopic       = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	WithdrawalTopic     = "0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65"
	DepositTopic        = "0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c"
	TransferName        = "Transfer"
	WithdrawalName      = "Withdrawal"
	DepositName         = "Deposit"
	ApprovalName        = "Approval"
	ApprovalIndexName   = "ApprovalIndex"
	ApprovalDataName    = "ApprovalData"
	ApprovalForAllName  = "ApprovalForAll"
	Permit2IndexName    = "Permit2"
	TransferIndexName   = "TransferIndex"
	WithdrawalIndexName = "WithdrawalIndex"
	DepositIndexName    = "DepositIndex"

	ABIs = `[
		{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":false,"name":"src","type":"address"},{"indexed":false,"name":"wad","type":"uint256"}],"name":"Withdrawal","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":false,"name":"dst","type":"address"},{"indexed":false,"name":"wad","type":"uint256"}],"name":"Deposit","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"name":"owner","type":"address"},{"indexed":true,"name":"spender","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Approval","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"name":"owner","type":"address"},{"indexed":true,"name":"spender","type":"address"},{"indexed":true,"name":"value","type":"uint256"}],"name":"ApprovalIndex","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"name":"owner","type":"address"},{"indexed":true,"name":"token","type":"address"},{"indexed":true,"name":"spender","type":"address"},{"indexed":false,"name":"value","type":"uint160"},{"indexed":false,"name":"expiration","type":"uint48"}],"name":"Permit2","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"name":"owner","type":"address"},{"indexed":true,"name":"spender","type":"address"},{"indexed":false,"name":"approved","type":"bool"}],"name":"ApprovalForAll","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":true,"name":"value","type":"uint256"}],"name":"TransferIndex","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"name":"src","type":"address"},{"indexed":false,"name":"wad","type":"uint256"}],"name":"WithdrawalIndex","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"name":"dst","type":"address"},{"indexed":false,"name":"wad","type":"uint256"}],"name":"DepositIndex","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":false,"name":"owner","type":"address"},{"indexed":false,"name":"spender","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"ApprovalData","type":"event"}
	]`
)

func Decode(log types.Log) (map[string]any, error) {
	event := map[string]any{}
	abiName := decodeWithTopic(log)
	if abiName == "" {
		return event, nil
	}

	eventAbi, err := abi.JSON(strings.NewReader(ABIs))
	if err != nil {
		return event, err
	}
	if _, ok := eventAbi.Events[abiName]; !ok {
		return event, errors.New("abi not found")
	}

	indexed := []abi.Argument{}
	for _, input := range eventAbi.Events[abiName].Inputs {
		if input.Indexed {
			indexed = append(indexed, input)
		}
	}
	if err := abi.ParseTopicsIntoMap(event, indexed, log.Topics[1:]); err != nil {
		return event, errors.New("unpack abi's topics is err: " + err.Error() + " on tx: " + log.TxHash.String())
	}
	if len(log.Data) > 0 {
		err = eventAbi.UnpackIntoMap(event, abiName, log.Data)
		if err != nil {
			return event, errors.New("unpack abi's data is err: " + err.Error() + " on tx: " + log.TxHash.String())
		}
	}
	return event, nil
}

func decodeWithTopic(log types.Log) string {
	topic0 := strings.ToLower(log.Topics[0].String())

	topicName := TopicMaps[topic0]

	switch topicName {
	case TransferName:
		if len(log.Topics) == 4 {
			return TransferIndexName
		}
	case WithdrawalName:
		if len(log.Topics) == 2 {
			return WithdrawalIndexName
		}
	case DepositName:
		if len(log.Topics) == 2 {
			return DepositIndexName
		}
	case ApprovalName:
		if len(log.Topics) == 1 {
			return ApprovalDataName
		}
		if len(log.Topics) == 4 {
			return ApprovalIndexName
		}
	}
	return topicName
}

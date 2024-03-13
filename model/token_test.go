package model

import (
	"fmt"
	"testing"

	"go-etl/config"
)

func TestGetCoinGeCkoPrices(t *testing.T) {
	config.SetupConfig("../config/config.dev.yaml")
	tokens := Tokens{
		Token{
			Address: "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
		},
		Token{
			Address: "0x2260fac5e5542a773aa44fbcfedf7c193bc2c599",
		},
	}
	fmt.Println(tokens.GetCoinGeCkoPrices())
}

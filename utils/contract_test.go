package utils

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"

	"go-etl/client"
	"go-etl/config"
)

func TestIsErc20Or721(t *testing.T) {
	config.SetupConfig("../config/config.dev.yaml")
	code, err := client.EvmClient().CodeAt(context.Background(), common.HexToAddress("0x6dd54ea98bf4a9a770f023a1d3dc091d91210f5c"), nil)
	if err != nil {
		logrus.Fatal(err)
	}
	fmt.Println(IsErc20Or721(Erc20Signatures, code, 5))
}

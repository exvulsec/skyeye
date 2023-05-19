package model

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/magiconair/properties/assert"

	"go-etl/client"
	"go-etl/config"
	"go-etl/utils"
)

func TestGetPush4Args(t *testing.T) {
	config.SetupConfig("../config/config.dev.yaml")
	code, err := client.EvmClient().CodeAt(context.Background(), common.HexToAddress("0xc667e8ac55590d140957df005ca0c2ef69698270"), nil)
	if err != nil {
		panic(err)
	}
	opCodeArgs := GetPushTypeArgs(code)
	args := GetPush4Args(opCodeArgs[utils.PUSH4])
	assert.Equal(t, args, []string{"finance", "init", "moboxBridge", "owner", "setFinance", "setSupportToken", "supportTokens", "transferOwnership", "0x{13}"})
}

func TestGetPush20Args(t *testing.T) {
	config.SetupConfig("../config/config.dev.yaml")
	code, err := client.EvmClient().CodeAt(context.Background(), common.HexToAddress("0xc667e8ac55590d140957df005ca0c2ef69698270"), nil)
	if err != nil {
		panic(err)
	}
	opCodeArgs := GetPushTypeArgs(code)
	if err != nil {
		panic(err)
	}
	addr := GetPush20Args("ethereum", opCodeArgs[utils.PUSH20])
	assert.Equal(t, addr, []string{"Null: 0xeee...eee"})

}

func TestIsErc20Or721(t *testing.T) {
	config.SetupConfig("../config/config.dev.yaml")
	code, err := client.EvmClient().CodeAt(context.Background(), common.HexToAddress("0xc667e8ac55590d140957df005ca0c2ef69698270"), nil)
	if err != nil {
		panic(err)
	}
	opCodeArgs := GetPushTypeArgs(code)
	if err != nil {
		panic(err)
	}
	args := GetPush4Args(opCodeArgs[utils.PUSH4])
	assert.Equal(t, utils.IsErc20Or721(utils.Erc20Signatures, args, 5), false)
}

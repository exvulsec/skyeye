package model

import (
	"context"
	"fmt"
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

func TestIsToken(t *testing.T) {
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
	assert.Equal(t, utils.IsToken(utils.Erc20Signatures, args, utils.Erc20SignatureThreshold), false)
}

func TestGetPushTypeArgs(t *testing.T) {
	type args struct {
		byteCode []byte
	}
	tests := []struct {
		name string
		args args
		want map[string][]string
	}{
		{
			name: "test case 1",
			args: args{byteCode: []byte{0x00, 0x00, 0x6d, 0x64, 0x64, 0x6e, 0x6f, 0x6d, 0x20, 0x3c, 0x3d, 0x20, 0x70, 0x72, 0x6f, 0x64, 0x31}},
			want: map[string][]string{"key": {"value"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss := GetPushTypeArgs(tt.args.byteCode)
			fmt.Println("ASCII:", ss[utils.LOGS])
		})
	}
}

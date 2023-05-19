package model

import (
	"testing"

	"github.com/magiconair/properties/assert"

	"go-etl/config"
	"go-etl/utils"
)

func TestGetPush4Args(t *testing.T) {
	config.SetupConfig("../config/config.dev.yaml")
	opstring, err := GetOpCodes("ethereum", "0xc667e8ac55590d140957df005ca0c2ef69698270")
	if err != nil {
		panic(err)
	}
	args := GetFuncSignatures(GetPush4Args(opstring))
	assert.Equal(t, args, []string{"finance", "init", "owner", "setFinance", "setSupportToken", "supportTokens", "transferOwnership", "0x{14}"})
}

func TestGetPush20Args(t *testing.T) {
	config.SetupConfig("../config/config.dev.yaml")
	opstring, err := GetOpCodes("ethereum", "0xc667e8ac55590d140957df005ca0c2ef69698270")
	if err != nil {
		panic(err)
	}
	args := GetPush20Args("ethereum", opstring)
	assert.Equal(t, args, []string{"Null: 0xeee...eee"})

}

func TestIsErc20Or721(t *testing.T) {
	config.SetupConfig("../config/config.dev.yaml")
	opstring, err := GetOpCodes("ethereum", "0xc667e8ac55590d140957df005ca0c2ef69698270")
	if err != nil {
		panic(err)
	}
	args := GetPush4Args(opstring)
	assert.Equal(t, utils.IsErc20Or721(utils.Erc20Signatures, args, 5), false)
}

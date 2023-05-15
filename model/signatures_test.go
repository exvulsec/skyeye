package model

import (
	"testing"

	"github.com/magiconair/properties/assert"

	"go-etl/config"
)

func TestOpenChainResponse_GetSignatures(t *testing.T) {
	config.SetupConfig("../config/config.dev.yaml")
	signatures, err := GetSignatures([]string{"0x06fdde03", "0x23b872dd", "0x00000000"})
	if err != nil {
		panic(err)
	}
	assert.Equal(t, signatures, []string{"name()", "transferFrom(address,address,uint256)", "0x{1}"})
}

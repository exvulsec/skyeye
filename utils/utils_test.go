package utils

import (
	"testing"

	"github.com/magiconair/properties/assert"

	"go-etl/config"
)

func TestGetBlockNumberFromDB(t *testing.T) {
	config.SetupConfig("../config/config.dev.yaml")
	block := GetBlockNumberFromDB("ethereum")
	assert.Equal(t, block, uint64(17482838))
}

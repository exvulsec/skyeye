package utils

import (
	"testing"

	"github.com/magiconair/properties/assert"

	"github.com/exvulsec/skyeye/config"
)

func TestGetBlockNumberFromDB(t *testing.T) {
	config.SetupConfig("../config/config.dev.yaml")
	block := GetBlockNumberFromDB("ethereum")
	assert.Equal(t, block, uint64(17482838))
}

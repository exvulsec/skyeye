package extractor

import (
	"testing"

	"github.com/exvulsec/skyeye/config"
)

func TestExtractBlocks(t *testing.T) {
	config.SetupConfig("../config/config.dev.yaml")
	te := NewTransactionExtractor(5)
	te.extractTransactions()
}

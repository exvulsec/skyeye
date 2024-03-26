package extractor

import (
	"testing"
)

func TestExtractBlocks(t *testing.T) {
	te := NewTransactionExtractor(5)
	te.Run()
}

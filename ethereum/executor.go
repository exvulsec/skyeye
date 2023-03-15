package ethereum

import "github.com/ethereum/go-ethereum/core/types"

type Executor interface {
	Run()
	ExtractByBlock(block types.Block) any
	Export()
	Enrich()
}

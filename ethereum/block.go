package ethereum

import (
	"context"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"

	"go-etl/client"
	"go-etl/config"
	"go-etl/utils"
)

func GetLatestBlocks() chan types.Block {
	blocks := make(chan types.Block, 100)
	go getLatestBlocks(blocks)
	return blocks
}

func getLatestBlocks(blocks chan types.Block) {
	lastBlockNumber := utils.ReadBlockNumberFromFile(config.Conf.ETLConfig.PreviousFile)

	headers := make(chan *types.Header, 10)
	sub, err := client.EvmClient().SubscribeNewHead(context.Background(), headers)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case err := <-sub.Err():
			log.Fatal(err)
		case header := <-headers:
			if lastBlockNumber != 0 {
				for lastBlockNumber+1 < header.Number.Int64() {
					block, err := client.EvmClient().BlockByNumber(context.Background(), big.NewInt(lastBlockNumber+1))
					if err != nil {
						log.Fatal(err)
					}
					blocks <- *block
					lastBlockNumber = lastBlockNumber + 1
				}
			}
			block, err := client.EvmClient().BlockByHash(context.Background(), header.Hash())
			if err != nil {
				log.Fatal(err)
			}
			blocks <- *block
			lastBlockNumber = header.Number.Int64()
		}
	}
}

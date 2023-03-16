package ethereum

import (
	"context"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

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
	for {
		lastBlockNumber := utils.ReadBlockNumberFromFile(config.Conf.ETLConfig.PreviousFile) + 1

		headers := make(chan *types.Header, 10)
		sub, err := client.EvmClient().SubscribeNewHead(context.Background(), headers)
		if err != nil {
			logrus.Infof("failed to subscribe to new head block: %v, retry after 5s", err)
			time.Sleep(5 * time.Second)
			continue
		}

		logrus.Infof("subscribed to new head")

		for {
			select {
			case err = <-sub.Err():
				logrus.Errorf("subscription error: %v\n", err)
				sub.Unsubscribe()
				break
			case header := <-headers:
				if lastBlockNumber != 0 {
					for lastBlockNumber < header.Number.Int64() {
						block, err := client.EvmClient().BlockByNumber(context.Background(), big.NewInt(lastBlockNumber))
						if err != nil {
							log.Fatal(err)
						}
						blocks <- *block
						lastBlockNumber = lastBlockNumber + 1
					}
				}
				block, err := client.EvmClient().BlockByHash(context.Background(), header.Hash())
				if err != nil {
					logrus.Fatalf("get the block by hash %s is err %v", block.Hash(), err)
				}
				blocks <- *block
				lastBlockNumber = header.Number.Int64()
			}
		}
	}
}

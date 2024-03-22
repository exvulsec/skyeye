package cmd

import (
	"github.com/spf13/cobra"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/ethereum"
	"github.com/exvulsec/skyeye/log"
)

var skyeyeCmd = &cobra.Command{
	Use:   "skyeye",
	Short: "watch the risk transaction on the chain",
	Run: func(cmd *cobra.Command, args []string) {
		configFile, _ := cmd.Flags().GetString("config")
		config.SetupConfig(configFile)
		log.InitLog(config.Conf.ETL.LogPath)

		workers, _ := cmd.Flags().GetInt("workers")
		chain, _ := cmd.Flags().GetString("chain")
		openAPIServer, _ := cmd.Flags().GetString("openapi_server")
		topicString, _ := cmd.Flags().GetString("topics")
		blockExecutor := ethereum.NewBlockExecutor(chain)
		var logExecutor ethereum.Executor
		if topicString != "" {
			logExecutor = ethereum.NewLogExecutor(chain, workers, ethereum.ConvertTopicsFromString(topicString))
		}
		executor := ethereum.NewTransactionExecutor(blockExecutor, logExecutor, chain, openAPIServer, workers, true)
		executor.Run()
	},
}

func skyEyeCmdInit() {
	skyeyeCmd.Flags().String("config", "", "set config file path")
	skyeyeCmd.Flags().Int("workers", 5, "batch call workers")
	skyeyeCmd.Flags().Int("batch_size", 50, "one batch call workers ")
	skyeyeCmd.Flags().String("chain", "ethereum", "chain name")
	skyeyeCmd.Flags().String("openapi_server", "http://localhost:8088", "open api server")
}

func init() {
	skyEyeCmdInit()
}

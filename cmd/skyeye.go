package cmd

import (
	"github.com/spf13/cobra"

	"go-etl/config"
	"go-etl/ethereum"
	"go-etl/log"
)

var skyeyeCmd = &cobra.Command{
	Use:   "sky-eye",
	Short: "watch the risk transaction on the chain",
	Run: func(cmd *cobra.Command, args []string) {
		configFile, _ := cmd.Flags().GetString("config")
		config.SetupConfig(configFile)
		log.InitLog(config.Conf.ETL.LogPath)

		workers, _ := cmd.Flags().GetInt("workers")
		batchSize, _ := cmd.Flags().GetInt("batch_size")
		chain, _ := cmd.Flags().GetString("chain")
		openAPIServer, _ := cmd.Flags().GetString("openapi_server")
		topicString, _ := cmd.Flags().GetString("topics")
		blockExecutor := ethereum.NewBlockExecutor(chain, batchSize, workers)
		var logExecutor ethereum.Executor
		if topicString != "" {
			logExecutor = ethereum.NewLogExecutor(chain, workers, ethereum.ConvertTopicsFromString(topicString))
		}
		executor := ethereum.NewTransactionExecutor(blockExecutor, logExecutor, chain, openAPIServer, workers, batchSize, true)
		executor.Run()
	},
}

func skyEyeCmdInit() {
	skyeyeCmd.Flags().String("config", "", "set config file path")
	skyeyeCmd.Flags().Int("workers", 5, "batch call workers")
	skyeyeCmd.Flags().Int("batch_size", 50, "one batch call workers ")
	skyeyeCmd.Flags().String("chain", "ethereum", "chain name")
	skyeyeCmd.Flags().String("openapi_server", "http://localhost:8088", "open api server")
	skyeyeCmd.Flags().String("topics", "", "filter the specified topics, split by comma")
	skyeyeCmd.Flags().String("log_table", "logs", "log table name")
}

func init() {
	etlCmd.AddCommand(skyeyeCmd)
	skyEyeCmdInit()
}

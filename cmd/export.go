package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go-etl/config"
	"go-etl/ethereum"
	"go-etl/log"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "export data from blockchain node",
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.Help(); err != nil {
			logrus.Panicf("failed to using help command, err is %v", err)
		}
	},
}

var txCmd = &cobra.Command{
	Use:   "txs",
	Short: "export latest tx from blockchain node",
	Run: func(cmd *cobra.Command, args []string) {
		configFile, _ := cmd.Flags().GetString("config")
		config.SetupConfig(configFile)
		log.InitLog(config.Conf.ETL.LogPath)
		workers, _ := cmd.Flags().GetInt("workers")
		isNastiff, _ := cmd.Flags().GetBool("is_nastiff")
		chain, _ := cmd.Flags().GetString("chain")
		openAPIServer, _ := cmd.Flags().GetString("openapi_server")
		topicString, _ := cmd.Flags().GetString("topics")
		blockExecutor := ethereum.NewBlockExecutor(chain)
		var logExecutor ethereum.Executor
		if topicString != "" {
			logExecutor = ethereum.NewLogExecutor(chain, workers, ethereum.ConvertTopicsFromString(topicString))
		}
		executor := ethereum.NewTransactionExecutor(blockExecutor, logExecutor, chain, openAPIServer, workers, isNastiff)
		executor.Run()
	},
}

func txCmdInit() {
	txCmd.Flags().String("config", "", "set config file path")
	txCmd.Flags().Uint64("tx_nonce", 0, "filter the less than nonce count txs, > 0 is available, default is 0")
	txCmd.Flags().Bool("is_nastiff", false, "filter nastiff txs")
	txCmd.Flags().Int("workers", 5, "batch call workers")
	txCmd.Flags().String("chain", "ethereum", "chain name")
	txCmd.Flags().String("openapi_server", "http://localhost:8088", "open api server")
	txCmd.Flags().String("topics", "", "filter the specified topics, split by comma")
}

func init() {
	exportCmd.AddCommand(txCmd)
	txCmdInit()
}

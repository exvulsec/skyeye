package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go-etl/config"
	"go-etl/ethereum"
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
		config.SetupConfig()
		nonce, _ := cmd.Flags().GetInt("tx_nonce")
		workers, _ := cmd.Flags().GetInt("workers")
		batchSize, _ := cmd.Flags().GetInt("batch_size")
		isCreationContract, _ := cmd.Flags().GetBool("creation_contract")
		writeRedis, _ := cmd.Flags().GetBool("write_redis")
		blocksCh := ethereum.NewBlockExecutor(batchSize, workers)
		executor := ethereum.NewTransactionExecutor(blocksCh, workers, batchSize, nonce, isCreationContract, writeRedis)
		executor.Run()
	},
}

func init() {
	exportCmd.AddCommand(txCmd)
	txCmd.Flags().StringVarP(&config.CfgPath, "config", "c", "", "set config file path")
	txCmd.Flags().Int("tx_nonce", 0, "filter the less than nonce count txs, > 0 is available, default is 0")
	txCmd.Flags().Bool("creation_contract", false, "filter the contract create txs")
	txCmd.Flags().Bool("write_redis", false, "redis txs to redis")
	txCmd.Flags().Int("workers", 2, "batch call workers")
	txCmd.Flags().Int("batch_size", 50, "one batch call workers ")
}

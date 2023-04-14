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
		nonce, _ := cmd.Flags().GetUint64("tx_nonce")
		workers, _ := cmd.Flags().GetInt("workers")
		batchSize, _ := cmd.Flags().GetInt("batch_size")
		isCreationContract, _ := cmd.Flags().GetBool("creation_contract")
		chain, _ := cmd.Flags().GetString("chain")
		tableName, _ := cmd.Flags().GetString("table_name")
		blockExecutor := ethereum.NewBlockExecutor(chain, batchSize, workers)
		executor := ethereum.NewTransactionExecutor(blockExecutor, chain, tableName, workers, batchSize, nonce, isCreationContract)
		executor.Run()
	},
}

func txCmdInit() {
	txCmd.Flags().StringVarP(&config.CfgPath, "config", "c", "", "set config file path")
	txCmd.Flags().Uint64("tx_nonce", 0, "filter the less than nonce count txs, > 0 is available, default is 0")
	txCmd.Flags().Bool("creation_contract", false, "filter the contract create txs")
	txCmd.Flags().Int("workers", 2, "batch call workers")
	txCmd.Flags().Int("batch_size", 50, "one batch call workers ")
	txCmd.Flags().String("chain", "ethereum", "chain name")
	txCmd.Flags().String("table_name", "txs", "table name")
}

func init() {
	exportCmd.AddCommand(txCmd)
	txCmdInit()
}

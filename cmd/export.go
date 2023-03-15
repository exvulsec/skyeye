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
		blocksCh := ethereum.GetLatestBlocks()
		executor := ethereum.NewTransactionExecutor(blocksCh)
		executor.Run()
	},
}

var contractCreationCmd = &cobra.Command{
	Use:   "contract-creation-txs",
	Short: "export create contract txs from blockchain node",
	Run: func(cmd *cobra.Command, args []string) {
		config.SetupConfig()
		blocksCh := ethereum.GetLatestBlocks()
		executor := ethereum.NewContractCreationExecutor(blocksCh)
		executor.Run()
	},
}

func init() {
	exportCmd.AddCommand(txCmd)
	exportCmd.AddCommand(contractCreationCmd)
	txCmd.Flags().StringVarP(&config.CfgPath, "config", "c", "", "set config file path")
	contractCreationCmd.Flags().StringVarP(&config.CfgPath, "config", "c", "", "set config file path")
}

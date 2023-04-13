package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"go-etl/config"
	"go-etl/datastore"
	"go-etl/ethereum"
)

var filterCmd = &cobra.Command{
	Use:   "filter",
	Short: "filter data from blockchain node",
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.Help(); err != nil {
			logrus.Panicf("failed to using help command, err is %v", err)
		}
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "filter logs by given topics from latest block",
	Run: func(cmd *cobra.Command, args []string) {
		config.SetupConfig()
		chain, _ := cmd.Flags().GetString("chain")
		tableName, _ := cmd.Flags().GetString("table_name")
		topics, _ := cmd.Flags().GetString("topics")
		logFilter := ethereum.NewLogFilter(chain, tableName, topics)
		logFilter.Run()
	},
}

func init() {
	filterCmd.AddCommand(logsCmd)
	logsCmd.Flags().StringVarP(&config.CfgPath, "config", "c", "", "set config file path")
	logsCmd.Flags().String("chain", "ethereum", "chain name")
	logsCmd.Flags().String("table_name", datastore.TableLogs, "table name")
	logsCmd.Flags().String("topics", "", "filter the specified topics, split by comma")
}

package cmd

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/executor"
	"github.com/exvulsec/skyeye/log"
)

var extractCMD = &cobra.Command{
	Use:   "extract",
	Short: "watch the risk transaction on the chain",
	Run: func(cmd *cobra.Command, args []string) {
		configFile, _ := cmd.Flags().GetString("config")
		if configFile != "" {
			if err := os.Setenv("CONFIG_PATH", configFile); err != nil {
				logrus.Panicf("set CONFIG_PATH's value to envoriment is err %v", err)
			}
		}
		log.InitLog(config.Conf.ETL.LogPath)

		executor := executor.NewBlockExecutor()
		executor.Execute()
	},
}

func extractCMDInit() {
	extractCMD.Flags().Int("workers", 5, "process the data workers' count")
}

func init() {
	extractCMDInit()
}

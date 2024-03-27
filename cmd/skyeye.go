package cmd

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/extractor"
	"github.com/exvulsec/skyeye/log"
)

var skyeyeCmd = &cobra.Command{
	Use:   "skyeye",
	Short: "watch the risk transaction on the chain",
	Run: func(cmd *cobra.Command, args []string) {
		configFile, _ := cmd.Flags().GetString("config")
		if configFile != "" {
			if err := os.Setenv("CONFIG_PATH", configFile); err != nil {
				logrus.Panicf("set CONFIG_PATH's value to envoriment is err %v", err)
			}
		}
		log.InitLog(config.Conf.ETL.LogPath)

		workers, _ := cmd.Flags().GetInt("workers")

		executor := extractor.NewTransactionExtractor(workers)
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

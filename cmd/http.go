package cmd

import (
	"github.com/spf13/cobra"

	"github.com/exvulsec/skyeye/config"
	"github.com/exvulsec/skyeye/log"
	"github.com/exvulsec/skyeye/server"
)

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "run http server",
	Run: func(cmd *cobra.Command, args []string) {
		config.SetupConfig("")
		log.InitLog(config.Conf.HTTPServer.LogPath)
		srv := server.NewHTTPServer()
		srv.Run()
	},
}

func init() {
	httpCmd.Flags().StringVarP(&config.CfgPath, "config", "c", "", "set config file path")
	httpCmd.Flags().StringVarP(&config.Env,
		"env",
		"e",
		"dev",
		"server environment type, available: dev, prod")
}

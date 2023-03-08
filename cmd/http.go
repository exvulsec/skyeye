package cmd

import (
	"github.com/spf13/cobra"

	"openapi/config"
	"openapi/server"
)

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "run http server",
	Run: func(cmd *cobra.Command, args []string) {
		config.SetupConfig()
		srv := server.NewHTTPServer()
		srv.Run()
	},
}

func setHTTPFlags() {
	httpCmd.Flags().StringVarP(&config.CfgPath, "config", "c", "", "set config file path")
	httpCmd.Flags().StringVarP(&config.Env,
		"env",
		"e",
		"dev",
		"server environment type, available: dev, prod")
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "openapi",
		Short: "openapi are multiple servers",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		panic(fmt.Errorf("execute cmd is err: %v", err))
	}
}

func init() {
	rootCmd.AddCommand(httpCmd)
	setHTTPFlags()
}

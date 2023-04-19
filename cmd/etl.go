package cmd

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	etlCmd = &cobra.Command{
		Use:   "etl",
		Short: "go etl is a blockchain etl tool",
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Help(); err != nil {
				logrus.Panicf("failed to using help command, err is %v", err)
			}
		},
	}
)

func Execute() {
	if err := etlCmd.Execute(); err != nil {
		panic(fmt.Errorf("execute cmd is err: %v", err))
	}
}

func init() {
	etlCmd.AddCommand(httpCmd)
	etlCmd.AddCommand(exportCmd)
	etlCmd.AddCommand(filterCmd)
}

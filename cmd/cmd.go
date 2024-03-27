package cmd

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var root = &cobra.Command{
	Use:   "skyeye",
	Short: "skyeye cmd",
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.Help(); err != nil {
			logrus.Panicf("failed to using help command, err is %v", err)
		}
	},
}

func Execute() {
	if err := root.Execute(); err != nil {
		panic(fmt.Errorf("execute cmd is err: %v", err))
	}
}

func init() {
	root.AddCommand(httpCmd)
	root.AddCommand(extractCMD)
}

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var logName string

func init() {
	RootCmd.PersistentFlags().StringVar(&logName, "log", "", "name (file path) of the log file")
	viper.BindPFlag("logName", RootCmd.PersistentFlags().Lookup("logname"))
}

var RootCmd = &cobra.Command{
	Use:   "t-logs",
	Short: "T-logs is a Tendermint debugging tool",
}

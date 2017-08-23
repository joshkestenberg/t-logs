package cmd

import (
	"log"

	"github.com/joshkestenberg/t-logs/reader"
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
	Long: `T-logs is the root command, and will, from a given Tendermint log file (must be a verbose log to use msgs or peers accurately), render a parseable log file with all parser-unfriendly lines removed.
  Requires flag: --log (file path)`,
}

func Parse() []reader.LogEntry {

	filename := logName

	file, err := reader.OpenLog(filename)
	if err != nil {
		log.Fatal(err)
	}

	renderFile, err := reader.RenderDoc(file)
	if err != nil {
		log.Fatal(err)
	}

	entries, err := reader.UnmarshalLines(renderFile)
	if err != nil {
		log.Fatal(err)
	}

	return entries
}

package cmd

import (
	"fmt"
	"log"
	"strconv"

	"github.com/joshkestenberg/t-logs/reader"

	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(StateCmd)
	RootCmd.AddCommand(MsgsCmd)
}

var StateCmd = &cobra.Command{
	Use:   "state",
	Short: "Get the state of a tendermint node at a particular time",
	Long: `For a given node at a given date and time, state will display height, round, step, proposal, block parts received, prevotes received, and precommits received.
  Takes two args: date (01-01) and time (00:00:00.000)`,
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) != 2 {
			log.Fatal("2 args required: date (01-01) and time (00:00:00.000)")
		}

		date := args[0]
		time := args[1]

		entries := Parse()

		status, err := reader.GetStatus(entries, date, time)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(status)
	},
}

var MsgsCmd = &cobra.Command{
	Use:   "msgs",
	Short: "See what messages a given node has received at intervals of a given length",
	Long: `For a given node at a given date and time, state will display height, round, step, proposal, block parts received, prevotes received, and precommits received.
  Takes two args: peer file path, duration in miliseconds (up to 59999), start date (01-01), start time (00:00:00.000), end date, and end time`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 6 {
			log.Fatal("6 args required: peer file path, duration in miliseconds (up to 59999), start date (01-01), start time (00:00:00.000), end date, and end time")
		}

		entries := Parse()

		peerFile := args[0]
		dur, err := strconv.Atoi(args[1])
		if err != nil {
			log.Fatal(err)
		}

		stD := args[2]
		stT := args[3]
		enD := args[4]
		enT := args[5]

		err = reader.GetMessages(entries, peerFile, dur, stD, stT, enD, enT)
		if err != nil {
			log.Fatal(err)
		}
	},
}

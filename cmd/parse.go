package cmd

import (
	"fmt"
	"log"
	"strconv"

	"github.com/joshkestenberg/t-logs/reader"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var order *bool
var commits *bool

func init() {
	RootCmd.AddCommand(StateCmd)
	RootCmd.AddCommand(MsgsCmd)
	RootCmd.AddCommand(PeersCmd)

	order = MsgsCmd.PersistentFlags().Bool("order", false, "add msgs indicating when logs are out of order")
	commits = MsgsCmd.PersistentFlags().Bool("commits", false, "add msgs indicating when blocks are committed")

	viper.BindPFlag("order", MsgsCmd.PersistentFlags().Lookup("order"))
	viper.BindPFlag("commits", MsgsCmd.PersistentFlags().Lookup("commits"))

}

var StateCmd = &cobra.Command{
	Use:   "state",
	Short: "Get the state of a tendermint node at a particular time",
	Long: `For a given node at a given date and time, state will display height, round, step, proposal, block parts received, prevotes received, precommits received, and also, prevotes and precommits sent.

	In vote arrays:
	X = received vote
	O = sent vote
	Y = received nil vote
	N = sent nil vote

  Takes thre args: peer file path, date (01-01), and time (00:00:00[.000 if you'd like more specificity])`,
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) != 3 {
			log.Fatal("3 args required: peer file path, date (01-01), and time (00:00:00[.000 if you'd like more specificity])")
		}

		peerFile := args[0]
		date := args[1]
		time := args[2]

		entries := Parse()

		status, err := reader.GetStatus(entries, date, time, peerFile)
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

	X = received vote
	O = sent vote
	Y = received nil vote
	N = sent nil vote

  Takes six args: peer file path, duration in miliseconds (up to 59999), start date (01-01), start time (00:00:00[.000 if you'd like more specificity]), end date, and end time.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 6 {
			log.Fatal("6 args required: peer file path, duration in miliseconds (up to 59999), start date (01-01), start time (00:00:00[.000 if you'd like more specificity]), end date, and end time")
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

		err = reader.GetMessages(entries, peerFile, dur, stD, stT, enD, enT, *order, *commits)
		if err != nil {
			log.Fatal(err)
		}
	},
}

var PeersCmd = &cobra.Command{
	Use:   "peers",
	Short: "Output a peer file",
	Long: `For a given node's log, peers will generate a list of all peer ip addresses (including it's own)
  Note: When debugging a cluster of nodes, only one peer file should be used so to preserve the order of the nodes represented.`,
	Run: func(cmd *cobra.Command, args []string) {
		entries := Parse()

		reader.FindIpsFromLog(entries)
	},
}

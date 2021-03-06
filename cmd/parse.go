package cmd

import (
	"fmt"
	"log"
	"strconv"

	"github.com/joshkestenberg/t-logs/filefuncs"
	"github.com/joshkestenberg/t-logs/reader"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var commits *bool

func init() {
	RootCmd.AddCommand(StateCmd)
	RootCmd.AddCommand(MsgsCmd)
	RootCmd.AddCommand(NodesCmd)

	commits = MsgsCmd.PersistentFlags().Bool("commits", false, "add msgs indicating when blocks are committed")

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

  Takes two args: date (01-01), and time (00:00:00[.000 if you'd like more specificity])`,
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) != 2 {
			log.Fatal("2 args required: date (01-01), and time (00:00:00[.000 if you'd like more specificity])")
		}

		date := args[0]
		time := args[1]

		file, err := filefuncs.OpenLog(logName)
		if err != nil {
			log.Fatal(err)
		}

		entries, err := filefuncs.UnmarshalLines(file)
		if err != nil {
			log.Fatal(err)
		}

		nodes, err := filefuncs.UnmarshalNodes()
		if err != nil {
			log.Fatal(err)
		}

		status, err := reader.GetStatus(entries, nodes, date, time)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(status)
	},
}

var MsgsCmd = &cobra.Command{
	Use:   "msgs",
	Short: "See what messages a given node has received at intervals of a given length over a given duration",
	Long: `For a given node over a given period of time, msgs will display a graphical representation of nodes from whom a message has been received, and whether or not a message has been sent by the given node.

	X = received msg
	O = sent msg

  Takes five args: duration in miliseconds (up to 59999), start date (01-01), start time (00:00:00[.000 if you'd like more specificity]), end date, and end time.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 5 {
			log.Fatal("5 args required: duration in miliseconds (up to 59999), start date (01-01), start time (00:00:00[.000 if you'd like more specificity]), end date, and end time")
		}

		file, err := filefuncs.OpenLog(logName)
		if err != nil {
			log.Fatal(err)
		}

		entries, err := filefuncs.UnmarshalLines(file)
		if err != nil {
			log.Fatal(err)
		}
		dur, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatal(err)
		}

		stD := args[1]
		stT := args[2]
		enD := args[3]
		enT := args[4]

		nodes, err := filefuncs.UnmarshalNodes()
		if err != nil {
			log.Fatal(err)
		}

		_, err = reader.GetMessages(entries, nodes, dur, stD, stT, enD, enT, *commits)
		if err != nil {
			log.Fatal(err)
		}
	},
}

var NodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Outputs nodes.json",
	Long: `Given a cluster of nodes' log files, nodes generates a json file with each node's name, ip, pubkey, and bit array index, and then renders a t-logs readable log file for each given log entitled "rendered_[log name...]".
  Note: log files must be given for all validator nodes reflected in logs.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Fatal("You must input log files for all validator nodes")
		}

		var names []string

		for _, arg := range args {
			openFile, err := filefuncs.OpenLog(arg)
			if err != nil {
				log.Fatal(err)
			}

			name, err := filefuncs.RenderDoc(openFile, arg)
			if err != nil {
				log.Fatal(err)
			}

			names = append(names, name)
		}

		filefuncs.GetNodes(names)
	},
}

package cmd

import (
	"bufio"
	"io/ioutil"
	"log"
	"os"
	"strings"

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

	file, err := OpenLog(logName)
	if err != nil {
		log.Fatal(err)
	}

	renderFile, err := RenderDoc(file)
	if err != nil {
		log.Fatal(err)
	}

	entries, err := UnmarshalLines(renderFile)
	if err != nil {
		log.Fatal(err)
	}

	return entries
}

//OpenLog opens log file
func OpenLog(filepath string) (*os.File, error) {
	return os.Open(filepath)
}

//RenderDoc removes blocks and spaces to prep doc for parse
func RenderDoc(file *os.File) (*os.File, error) {
	var err error

	fileStat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	name := fileStat.Name()
	renderName := "rendered_" + name

	if _, err := os.Stat("./" + renderName); err == nil {
		return os.OpenFile(renderName, os.O_RDWR|os.O_APPEND, 0600)
	}

	err = ioutil.WriteFile(renderName, nil, 0600)
	if err != nil {
		return nil, err
	}

	renderFile, err := os.OpenFile(renderName, os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		str := scanner.Text()
		if strings.Contains(str, `|`) {
			if strings.Contains(str, "Block{") {
				str = str + "}"
			}
			_, err = renderFile.WriteString(str + "\n")
			if err != nil {
				return nil, err
			}
		}
	}

	return os.OpenFile(renderName, os.O_RDWR|os.O_APPEND, 0600)
}

//UnmarshalLines converts given lines to an array of structs
func UnmarshalLines(file *os.File) ([]reader.LogEntry, error) {
	var str string
	var err error
	var entries []reader.LogEntry

	//intialize scanner and scan each line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		//scan each applicable line, make struct, add struct to []struct
		str = scanner.Text()
		entry, err := reader.UnmarshalLine(str)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, err
}

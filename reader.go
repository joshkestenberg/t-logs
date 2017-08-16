package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
)
import "errors"

//LogEntry defines params for eventual json
type LogEntry struct {
	Level   string            `json:"level"`
	Date    string            `json:"date"`
	Time    string            `json:"time"`
	Descrip string            `json:"descrip"`
	Module  string            `json:"module"`
	Other   map[string]string `json:"other"`
}

type Status struct {
	Height     int
	Round      int
	Step       string
	Proposal   string
	BlockParts []string
	PreVotes   []string
	PreCommits []string
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
			_, err = renderFile.WriteString(str + "\n")
			if err != nil {
				return nil, err
			}
		}
	}

	return os.OpenFile(renderName, os.O_RDWR|os.O_APPEND, 0600)
}

//UnmarshalLine takes one line at a time and outputs a struct
func UnmarshalLine(line string) (LogEntry, error) {
	var entry LogEntry
	var err error

	//parse for level
	levelParse := strings.SplitAfterN(line, "[", 2)
	entry.Level = strings.Replace(levelParse[0], "[", "", 1)

	//parse for date and time
	datetimeParse := strings.SplitAfterN(levelParse[1], "]", 2)

	entry.Date = strings.Split(datetimeParse[0], "|")[0]
	entry.Time = strings.Replace(strings.Split(datetimeParse[0], "|")[1], "]", "", 1)

	//parse for descrip
	descripParse := strings.Split(datetimeParse[1], "module")
	descrip := descripParse[0]

	//if the line describes a block, it needs to be specially processed
	if strings.Contains(descrip, "Block{") {
		entry.Module = "consensus"

		if strings.Contains(descrip, "signed proposal block") {
			entry.Descrip = "Signed proposal block"
		} else {
			entry.Descrip = "Block"
		}
		return entry, err
	} else {
		//set Descrip
		entry.Descrip = strings.TrimSpace(descrip)
	}

	//parse for all other entries
	mapParse := strings.Split(descripParse[1], "=")[1:]
	//set key to module for first iteration of loop (always module) and create empty map
	key := "module"
	m := make(map[string]string)

	//iterate over other entries and add them to map
	for _, element := range mapParse {
		var valKey []string

		if strings.Contains(element, `"`) {
			valKey = strings.Split(element, `"`)[1:]
		} else {
			valKey = strings.SplitN(element, " ", 2)
		}

		//set Value
		value := strings.TrimSpace(valKey[0])

		if key == "module" && len(valKey) > 1 {
			entry.Module = value
			key = strings.TrimSpace(valKey[1])
		} else if key == "module" {
			entry.Module = value
		} else if key != "module" && len(valKey) > 1 {
			m[key] = value
			key = strings.TrimSpace(valKey[1])
		} else if key != "module" {
			m[key] = value
		}
	}

	// add map to entry struct
	entry.Other = m

	return entry, err
}

//UnmarshalLines converts given lines to an array of structs
func UnmarshalLines(file *os.File) ([]LogEntry, error) {
	var str string
	var err error
	var entries []LogEntry

	//intialize scanner and scan each line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		//scan each applicable line, make struct, add struct to []struct
		str = scanner.Text()
		entry, err := UnmarshalLine(str)
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

func getMessages(entries []LogEntry, dur int) error {
	var trackTime int
	var err error

	if dur < 2 || dur > 59999 {
		return errors.New("duration must be greater than 2 and lesser than 59999")
	}

	first := true

	peers := findPeers(entries)

	var msgs []string

	for i := 0; i < len(peers); i++ {
		msgs = append(msgs, "_")
	}

	for _, entry := range entries {

		tParse := strings.Split(entry.Time, ":")
		timeParse := strings.Split(tParse[2], ".")
		mili := timeParse[0] + timeParse[1]

		time, err := strconv.Atoi(mili)
		if err != nil {
			return err
		}

		if first == true {
			trackTime = time
			first = false
		}

		if time > trackTime && (time-trackTime) >= dur {
			fmt.Println(entry.Time, msgs)
			//update trackTime
			trackTime = time
			//reset peers
			for i := 0; i < len(peers); i++ {
				msgs[i] = "_"
			}
		} else if time < trackTime && (60000-trackTime+time) >= dur {
			fmt.Println(entry.Time, msgs)
			//update trackTime
			trackTime = time
			//reset peers
			for i := 0; i < len(peers); i++ {
				msgs[i] = "_"
			}
		}

		//processing logic for received msgs
		if entry.Descrip == "Receive" {
			for i, peer := range peers {
				if entry.Other["src"] == peer {
					msgs[i] = "X"
				}
			}
		}
	}
	return err
}

//this belongs to getMessages
func findPeers(entries []LogEntry) []string {
	var peers []string
	var add bool

	for _, entry := range entries {
		add = true
		if entry.Descrip == "Receive" {
			for _, peer := range peers {
				if peer == entry.Other["src"] {
					add = false
				}
			}
			if add == true {
				peers = append(peers, entry.Other["src"])
			}
		}
	}
	return peers
}

//********************************************************************

func main() {

	args := os.Args

	logName := args[1]
	dur, err := strconv.Atoi(args[2])
	if err != nil {
		log.Fatal(err)
	}

	file, err := OpenLog(logName)
	if err != nil {
		log.Fatal(err)
	}

	renderFile, err := RenderDoc(file)

	entries, err := UnmarshalLines(renderFile)
	if err != nil {
		log.Fatal(err)
	}

	err = getMessages(entries, dur)
	if err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
)

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
func RenderDoc(file *os.File, stLn int, endLn int) (*os.File, error) {
	var err error
	var count int

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
		//start and end line logic
		count += 1
		if count < stLn {
			continue
		} else if count > endLn {
			break
		}

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

func getMessages(entries []LogEntry, peerFilename string, dur int) error {
	var trackTime int
	var prevMinute int
	var err error

	first := true

	peers, err := findPeers(peerFilename)
	if err != nil {
		return err
	}

	myIp := findMyIP(entries)

	var msgs []string

	for i := 0; i < len(peers); i++ {
		msgs = append(msgs, "_")
	}

	for _, entry := range entries {

		tParse := strings.Split(entry.Time, ":")
		timeParse := strings.Split(tParse[2], ".")
		mili := timeParse[0] + timeParse[1]

		minute, err := strconv.Atoi(tParse[1])
		if err != nil {
			return err
		}
		time, err := strconv.Atoi(mili)
		if err != nil {
			return err
		}

		if first == true {
			trackTime = time
			prevMinute = minute
			first = false
		}

		if (time > trackTime && (time-trackTime) >= dur) || (time < trackTime && (60000-trackTime+time) >= dur) || (minute == (prevMinute+1) && time > trackTime) || (minute >= (prevMinute + 2)) {
			fmt.Println(entry.Time, msgs)
			//update trackTime and prevMinute
			trackTime = time
			prevMinute = minute
			//reset peers
			for i := 0; i < len(peers); i++ {
				msgs[i] = "_"
			}
		} else if reflect.DeepEqual(entry, entries[len(entries)-1]) {
			fmt.Println(entry.Time, msgs)
		}

		//processing logic for received msgs
		if entry.Descrip == "Receive" {
			peerParse := strings.Split(entry.Other["src"], "}")[0]
			ip := strings.Split(peerParse, "{")[2]

			for i, peer := range peers {
				if myIp == peer {
					msgs[i] = "O"
				} else if ip == peer {
					msgs[i] = "X"
				}
			}
		}
	}
	return err
}

//these belong to getMessages
func findPeers(peerFilename string) ([]string, error) {
	var peers []string

	file, err := OpenLog(peerFilename)
	if err != nil {
		return peers, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		str := scanner.Text()
		peers = append(peers, str)
	}
	return peers, err
}

//This is just utilitarian script to get a list of ip's from the log file
func findIpsFromLog(entries []LogEntry) {
	var peers []string
	var add bool

	for _, entry := range entries {
		add = true
		if entry.Descrip == "Receive" {
			for _, peer := range peers {
				if entry.Other["src"] == peer {
					add = false
				}
			}
			if add == true {
				peers = append(peers, entry.Other["src"])
			}
		}
	}

	fmt.Println(peers)
}

func findMyIP(entries []LogEntry) string {
	var ip string

	for _, entry := range entries {
		if entry.Descrip == "Starting DefaultListener" {
			ipParse := strings.Split(entry.Other["impl"], "@")[1]
			ip = strings.Replace(ipParse, ")", "", 1)
			break
		}
	}
	return ip
}

//********************************************************************

func main() {
	//set up vars from args
	args := os.Args

	logName := args[1]
	peerFilename := args[2]
	stLn, err := strconv.Atoi(args[3])
	if err != nil {
		log.Fatal(err)
	}
	endLn, err := strconv.Atoi(args[4])
	if err != nil {
		log.Fatal(err)
	}
	dur, err := strconv.Atoi(args[5])
	if err != nil {
		log.Fatal(err)
	}

	file, err := OpenLog(logName)
	if err != nil {
		log.Fatal(err)
	}

	//make sure user input is valid
	if stLn <= 0 || endLn < stLn {
		log.Fatal(errors.New("make sure start line is greater than zero, and lesser than end line"))
	} else if dur < 2 || dur > 59999 {
		log.Fatal(errors.New("duration must be greater than 2 and lesser than 59999"))
	}

	renderFile, err := RenderDoc(file, stLn, endLn)
	if err != nil {
		log.Fatal(err)
	}

	entries, err := UnmarshalLines(renderFile)
	if err != nil {
		log.Fatal(err)
	}

	err = getMessages(entries, peerFilename, dur)
	if err != nil {
		log.Fatal(err)
	}
}

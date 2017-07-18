package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

type LogEntry struct {
	Level   string            `json:"level"`
	Date    string            `json:"date"`
	Time    string            `json:"time"`
	Descrip string            `json:"descrip"`
	Module  string            `json:"module"`
	Other   map[string]string `json:"other"`
}

//open log file
func OpenLog(filepath string) (*os.File, error) {
	return os.Open("./tendermint.log")
}

//generate and open json file
func InitJson(filename string) (*os.File, error) {

	err := ioutil.WriteFile(filename, nil, 0600)
	if err != nil {
		return nil, err
	}

	return os.OpenFile(filename, os.O_RDWR|os.O_APPEND, 0600)
}

func UnmarshalLines(startLn int, endLn int, file *os.File) ([]LogEntry, error) {
	count := 1
	block := false
	var str string
	var entry LogEntry
	var err error
	var entries []LogEntry

	//basic error handling
	if startLn < 1 || startLn > endLn {
		err = fmt.Errorf("startLn must be greater than 1 and less than endLn; your input [startLn: %d, endLn: %d]", startLn, endLn)
		return nil, err
	}

	//intialize scanner and scan each line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		//var for full line of text
		if count >= startLn && count <= endLn {
			str = scanner.Text()
		} else if count < startLn {
			count++
			continue
		} else if count > endLn {
			break
		}

		//skip over blank lines
		if str == "" {
			count++
			continue
		}

		//check if inside a block
		if !strings.Contains(str, "|") {
			block = true
			count++
			continue
		} else if block == true && strings.Contains(str, "}#") && strings.Contains(str, "module") {
			block = false
			count++
			continue
		}

		//parse for level
		levelParse := strings.SplitAfterN(str, "[", 2)
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
			entries = append(entries, entry)
			count++
			continue
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

		count++
		entries = append(entries, entry)
		if count > endLn {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, err
}

//func to write json to file
func MarshalJson(entries []LogEntry, jsonLog *os.File) error {
	var err error

	for i := 0; i < len(entries); i++ {
		jsonLogEntry, err := json.Marshal(entries[i])
		if err != nil {
			return err
		}

		_, err = jsonLog.WriteString(string(jsonLogEntry) + "\n")
		if err != nil {
			return err
		}
	}

	return err
}

// set block bool to false in order to run non-block processing by default
// var block bool = false

func main() {

	file, err := OpenLog("./tendermint.log")
	if err != nil {
		log.Fatal(err)
	}

	jsonLog, err := InitJson("log.json")
	if err != nil {
		log.Fatal(err)
	}

	entries, err := UnmarshalLines(1, 50650, file)
	if err != nil {
		log.Fatal(err)
	}

	err = MarshalJson(entries, jsonLog)
	if err != nil {
		log.Fatal(err)
	}

}

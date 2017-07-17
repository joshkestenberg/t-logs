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

//func to write json to file
func writeJson(entry LogEntry, jsonLog *os.File) error {

	jsonLogEntry, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = jsonLog.WriteString(string(jsonLogEntry) + "\n")
	if err != nil {
		return err
	}

	return err
}

func openLog(filepath string) *os.File {
	//open log file
	file, err := os.Open("./tendermint.log")
	if err != nil {
		log.Fatal(err)
	}
	return file
}

// set block bool to false in order to run non-block processing by default
var block bool = false

func main() {
	//generate and open json file
	err := ioutil.WriteFile("log.json", nil, 0600)
	if err != nil {
		log.Fatal(err)
	}

	jsonLog, err := os.OpenFile("log.json", os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		log.Fatal(err)
	}

	defer jsonLog.Close()

	file := openLog("./tendermint.log")

	//intialize scanner and scan each line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		//var for each full line of text
		str := scanner.Text()

		//skip over blank lines
		if str == "" {
			continue
		}

		//verify we're not inside a block
		if block == false {

			//init log struct
			var entry LogEntry

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
			//if the proceeding lines make up a block, we must process them differently by setting block to true
			if strings.Contains(descrip, "Block{") {
				entry.Module = "consensus"

				if strings.Contains(descrip, "signed proposal block") {
					entry.Descrip = "Signed proposal block"
				} else {
					entry.Descrip = "Block"
				}
				block = true

				//write json
				writeJson(entry, jsonLog)

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
				fmt.Println(valKey)
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

			//write json
			writeJson(entry, jsonLog)
		} else {
			// keep scanning and skipping lines until end of block
			if strings.Contains(str, "}#") && strings.Contains(str, "module") {
				block = false
				continue
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

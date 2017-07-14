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
	Level   string
	Date    string
	Time    string
	Descrip string
	Other   map[string]string
}

//func to write json to file
func writeJson(entry LogEntry) {
	log, err := os.OpenFile("log.json", os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		panic(err)
	}

	defer log.Close()

	jsonLogEntry, err := json.Marshal(entry)
	if err != nil {
		panic(err)
	}

	_, err = log.WriteString(string(jsonLogEntry) + "\n")
	if err != nil {
		panic(err)
	}
}

// set block bool to false in order to run non-block processing by default
var block bool = false

func main() {
	//generate json file
	ioutil.WriteFile("log.json", nil, 0600)

	//open log file
	file, err := os.Open("./tendermint.log")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	//intialize scanner and scan each line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		//var for each full line of text
		str := scanner.Text()

		//hacky fix to temporarily replace char that was causing problems
		modStr := strings.Replace(str, "[]", "sub", -1)
		fmt.Println(modStr)

		//skip over blank lines
		if modStr == "" {
			continue
		}

		//verify we're not inside a block
		if block == false {

			//init log struct
			var entry LogEntry

			//parse for level
			levelParse := strings.Split(modStr, "[")
			entry.Level = levelParse[0]

			//parse for date and time
			datetimeParse := strings.Split(levelParse[1], "]")
			entry.Date = strings.Split(datetimeParse[0], "|")[0]
			entry.Time = strings.Split(datetimeParse[0], "|")[1]

			//parse for descrip
			descripParse := strings.Split(datetimeParse[1], "module")

			descrip := descripParse[0]
			//if the proceeding lines make up a block, we must process them differently by setting block to true
			if strings.Contains(descrip, "Block{") {
				m := make(map[string]string)
				m["module"] = "consensus"
				entry.Descrip = "Block"

				block = true

				//write json
				writeJson(entry)

				continue
			} else {
				//set Descrip while replacing previously subbed []
				entry.Descrip = strings.Replace(descrip, "sub", "[]", -1)
			}

			//parse for all other entries
			mapParse := strings.Split(descripParse[1], "=")[1:]

			//set key to module for first iteration of loop (always module) and create empty map
			key := "module"
			m := make(map[string]string)
			//iterate over other entries and add them to map
			for _, element := range mapParse {
				valKey := strings.SplitN(element, " ", 2)
				//set Value while replacing previously subbed []
				value := strings.Replace(valKey[0], "sub", "[]", -1)

				m[key] = value

				if len(valKey) > 1 {
					key = valKey[1]
				}
			}

			// add map to entry struct
			entry.Other = m

			//write json
			writeJson(entry)
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

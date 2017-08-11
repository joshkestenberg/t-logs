package main

import (
	"C"
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
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

//InitJSON generates and opens json file
func InitJSON(filename string) (*os.File, error) {

	err := ioutil.WriteFile(filename, nil, 0600)
	if err != nil {
		return nil, err
	}

	return os.OpenFile(filename, os.O_RDWR|os.O_APPEND, 0600)
}

//RenderDoc removes blocks and spaces to prep doc for parse
func RenderDoc(file *os.File, startLn int, endLn int) (*os.File, error) {
	count := 1
	var err error

	//basic error handling
	if startLn < 1 || startLn > endLn {
		err = fmt.Errorf("startLn must be greater than 1 and less than endLn; your input [startLn: %d, endLn: %d]", startLn, endLn)
		return nil, err
	}

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
		if count <= endLn && count >= startLn {
			if strings.Contains(str, `|`) {
				_, err = renderFile.WriteString(str + "\n")
				if err != nil {
					return nil, err
				}
			}
			count++
		} else if count < startLn {
			count++
		} else {
			break
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

//MarshalJSON converts struct to json and writes to file
func MarshalJSON(entries []LogEntry, jsonLog *os.File) error {
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

//ParseJSON uses JSON query to find relevant data
func getStatus(entries []LogEntry) (Status, error) {
	var err error
	var status Status
	var intVals int
	var bpArr []string
	var pvArr []string
	var pcArr []string
	var myNode int

	bpUnique := true
	pvUnique := true
	pcUnique := true
	status.Proposal = "no"

	ln := 0

	for _, entry := range entries {
		ln += 1
		//find number of validators
		if val, ok := entry.Other["localPV"]; ok {
			vals := strings.Split(val, "{")[1]
			stringVals := strings.Split(vals, ":")[0]
			intVals, err = strconv.Atoi(stringVals)
			if err != nil {
				return status, err
			}
		}

		//find my own node's vote number
		if entry.Descrip == "Signed and pushed vote" {
			vote := entry.Other["vote"]

			temp := strings.Split(vote, "{")[1]
			i, err := strconv.Atoi(strings.Split(temp, ":")[0])
			if err != nil {
				return status, err
			}

			myNode = i
		}

		//check for new round; set HRS, and reset votes if new round
		if strings.Contains(entry.Descrip, "enter") && !strings.Contains(entry.Descrip, "Invalid") {

			descrip := entry.Descrip

			if strings.Contains(descrip, "enterNewRound") {
				hrs := strings.Split(descrip, " ")[2]
				hrsArr := strings.Split(hrs, "/")

				status.Height, err = strconv.Atoi(hrsArr[0])
				if err != nil {
					return status, err
				}
				status.Round, err = strconv.Atoi(hrsArr[1])
				if err != nil {
					return status, err
				}
				//set new round and reset parameters
				status.Step = "NewRound"
				fmt.Println("ENTER NEW ROUND!")
				fmt.Println(ln)

				status.Proposal = "No"

				status.BlockParts = status.BlockParts[:0]
				bpUnique = true

				status.PreVotes = status.PreVotes[:0]
				pvUnique = true

				status.PreCommits = status.PreCommits[:0]
				pcUnique = true

				for i := 0; i < intVals; i++ {
					status.PreVotes = append(status.PreVotes, "_")
					status.PreCommits = append(status.PreCommits, "_")
				}

			} else if strings.Contains(descrip, "enterPropose") {
				status.Step = "Propose"
				fmt.Println("ENTER PROPOSE!")
				fmt.Println(ln)

			} else if strings.Contains(descrip, "enterPrevote") {
				status.Step = "Prevote"
				fmt.Println("ENTER PREVOTE!")
				fmt.Println(ln)

			} else if strings.Contains(descrip, "enterPrecommit") {
				status.Step = "Precommit"
				fmt.Println("ENTER PRECOMMIT!")
				fmt.Println(ln)

			} else if strings.Contains(descrip, "enterCommit") {
				status.Step = "Commit"
				fmt.Println("ENTER COMMIT!")
				fmt.Println(ln)

			}
		}
		//check for proposal
		if entry.Descrip == "Received complete proposal block" {
			height, err := strconv.Atoi(entry.Other["height"])
			if err != nil {
				return status, err
			}
			if height == status.Height {
				status.Proposal = "yes"
				fmt.Println("PROP!")
				fmt.Println(ln)
			}
		} else if entry.Descrip == "Signed proposal" {
			status.Proposal = "yes"
			fmt.Println("PROP from me!")
			fmt.Println(ln)
		}
		//check for block parts
		if entry.Descrip == "Receive" && strings.Contains(entry.Other["msg"], "BlockPart") {
			blockPart := entry.Other["msg"]

			for _, part := range bpArr {
				if part == blockPart {
					bpUnique = false
					break
				} else {
					bpUnique = true
				}
			}

			if bpUnique == true {
				bpArr = append(bpArr, blockPart)
				status.BlockParts = append(status.BlockParts, "X")
				fmt.Println("UNIQUE BP!")
				fmt.Println(ln)

			}
		}
		//check for votes from unique validators, and add new validators to validator set
		if entry.Descrip == "Receive" && strings.Contains(entry.Other["msg"], "Vote Vote") {
			vote := entry.Other["msg"]

			if strings.Contains(vote, "Prevote") {
				for _, pv := range pvArr {
					if pv == vote {
						pvUnique = false
						break
					} else {
						pvUnique = true
					}
				}

				if pvUnique == true {
					pvArr = append(pvArr, vote)

					temp := strings.Split(vote, "{")[1]
					i, err := strconv.Atoi(strings.Split(temp, ":")[0])
					if err != nil {
						return status, err
					}
					fmt.Println("UNIQUE PV!")
					fmt.Println(ln)

					status.PreVotes[i] = "X"
				}
			}

			if strings.Contains(vote, "Precommit") {
				for _, pc := range pcArr {
					if pc == vote {
						pcUnique = false
						break
					} else {
						pcUnique = true
					}
				}

				if pcUnique == true {
					pcArr = append(pcArr, vote)

					temp := strings.Split(vote, "{")[1]
					i, err := strconv.Atoi(strings.Split(temp, ":")[0])
					if err != nil {
						return status, err
					}
					fmt.Println("UNIQUE PC!")
					fmt.Println(ln)

					status.PreCommits[i] = "X"
				}
			}
		}
		//check for own vote
		if entry.Descrip == "Signed and pushed vote" {
			if status.Step == "Prevote" {
				status.PreVotes[myNode] = "X"
			} else if status.Step == "Precommit" {
				status.PreCommits[myNode] = "X"
			}
		}
	}

	return status, err
}

func main() {

	args := os.Args

	logName := args[1]
	jsonName := args[2]
	startLn, err := strconv.Atoi(args[3])
	if err != nil {
		log.Fatal(err)
	}
	endLn, err := strconv.Atoi(args[4])
	if err != nil {
		log.Fatal(err)
	}

	file, err := OpenLog(logName)
	if err != nil {
		log.Fatal(err)
	}

	blankJsonLog, err := InitJSON(jsonName)
	if err != nil {
		log.Fatal(err)
	}

	renderFile, err := RenderDoc(file, startLn, endLn)

	entries, err := UnmarshalLines(renderFile)
	if err != nil {
		log.Fatal(err)
	}

	err = MarshalJSON(entries, blankJsonLog)
	if err != nil {
		log.Fatal(err)
	}

	status, err := getStatus(entries)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(status)

}

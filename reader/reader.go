package reader

import (
	"bufio"
	"fmt"
	"io/ioutil"
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

//GetStatus parses log entries for relevant data (see below for all relevant fucntions)
func GetStatus(entries []LogEntry, date string, time string) (Status, error) {
	var err error

	var status Status

	var bpArr []string
	var pvArr []string
	var pcArr []string

	var numVals int
	var myNode int

	var watch = false

	status.Proposal = "no"

	for _, entry := range entries {
		//this logic ensures that we get all entries at a given time rather than just the first
		if entry.Date == date && entry.Time == time {
			watch = true
		}

		if watch == true && entry.Time != time {
			break
		}

		if _, ok := entry.Other["localPV"]; ok {
			numVals, err = findNumVals(entry)
			if err != nil {
				return status, err
			}
		}

		if entry.Descrip == "Signed and pushed vote" {
			myNode, err = findMyNode(entry)
			if err != nil {
				return status, err
			}
		}

		if strings.Contains(entry.Descrip, "enter") && !strings.Contains(entry.Descrip, "Invalid") {
			status, err = newStep(status, entry, numVals)
			if err != nil {
				return status, err
			}
		}

		if entry.Descrip == "Received complete proposal block" {
			status, err = checkProp(status, entry)
			if err != nil {
				return status, err
			}
		}

		if entry.Descrip == "Receive" && strings.Contains(entry.Other["msg"], "BlockPart") {
			status, bpArr, err = checkBlock(status, entry, bpArr)
			if err != nil {
				return status, err
			}
		}

		if entry.Descrip == "Receive" && strings.Contains(entry.Other["msg"], "Vote Vote") {
			status, pvArr, pcArr, err = checkVotes(status, entry, pvArr, pcArr)
			if err != nil {
				return status, err
			}
		}

		if entry.Descrip == "Signed and pushed vote" {
			status, err = myVote(status, entry, myNode)
			if err != nil {
				return status, err
			}
		}
	}
	return status, err
}

func GetMessages(entries []LogEntry, peerFilename string, dur int, stD string, stT string, enD string, enT string) error {
	var trackTime int
	var prevMinute int
	var prevHour int
	var prevMonth int
	var prevDay int
	var parse bool
	var watch bool
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
		if entry.Date == stD && entry.Time == stT && parse == false {
			parse = true
		}

		if entry.Date == enD && entry.Time == enT {
			watch = true
		}

		if watch == true && entry.Time != enT {
			fmt.Println(entry.Time, msgs)
			break
		}

		if parse == true {
			tParse := strings.Split(entry.Time, ":")
			timeParse := strings.Split(tParse[2], ".")
			mili := timeParse[0] + timeParse[1]

			minute, err := strconv.Atoi(tParse[1])
			if err != nil {
				return err
			}
			hour, err := strconv.Atoi(tParse[0])
			if err != nil {
				return err
			}

			dateParse := strings.Split(entry.Date, "-")
			month, err := strconv.Atoi(dateParse[0])
			if err != nil {
				return err
			}
			day, err := strconv.Atoi(dateParse[1])
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

			//this if/else statement determines when to return a statement, accounting for the turnover of minutes, hours, days, months, years
			if (time > trackTime && (time-trackTime) >= dur) || (time < trackTime && (60000-trackTime+time) >= dur) { //duration elapses normally
				fmt.Println(entry.Time, msgs)
				//update trackers
				trackTime = time
				prevMinute = minute
				prevHour = hour
				prevDay = day
				prevMonth = month
				//reset peers
				for i := 0; i < len(peers); i++ {
					msgs[i] = "_"
				}
			} else if (minute == (prevMinute+1) && time > trackTime) || minute >= (prevMinute+2) { //minute rolls over
				fmt.Println("good")
				fmt.Println(entry.Time, msgs)
				//update trackers
				trackTime = time
				prevMinute = minute
				prevHour = hour
				prevDay = day
				prevMonth = month
				//reset peers
				for i := 0; i < len(peers); i++ {
					msgs[i] = "_"
				}
			} else if hour == (prevHour+1) && ((60000-trackTime+time) >= dur || time > trackTime) { //hour rolls over
				fmt.Println(entry.Time, msgs)
				//update trackers
				trackTime = time
				prevMinute = minute
				prevHour = hour
				prevDay = day
				prevMonth = month
				//reset peers
				for i := 0; i < len(peers); i++ {
					msgs[i] = "_"
				}
			} else if day == (prevDay+1) && ((60000-trackTime+time) >= dur || time > trackTime) { //day rolls over
				fmt.Println(entry.Time, msgs)
				//update trackers
				trackTime = time
				prevMinute = minute
				prevHour = hour
				prevDay = day
				prevMonth = month
				//reset peers
				for i := 0; i < len(peers); i++ {
					msgs[i] = "_"
				}
			} else if (month == (prevMonth+1) || month < prevMonth) && ((60000-trackTime+time) >= dur || time > trackTime) { //month rolls over
				fmt.Println(entry.Time, msgs)
				//update trackers
				trackTime = time
				prevMinute = minute
				prevHour = hour
				prevDay = day
				prevMonth = month
				//reset peers
				for i := 0; i < len(peers); i++ {
					msgs[i] = "_"
				}
			} else if reflect.DeepEqual(entry, entries[len(entries)-1]) { //log ends
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
	}
	return err
}

//BELOW FUNCTIONS BELONG TO getStatus ^^

//find number of validators
func findNumVals(entry LogEntry) (int, error) {
	var err error

	vals := strings.Split(entry.Other["localPV"], "{")[1]
	stringVals := strings.Split(vals, ":")[0]
	numVals, err := strconv.Atoi(stringVals)
	if err != nil {
		return numVals, err
	}

	return numVals, err
}

//find my own node's vote number
func findMyNode(entry LogEntry) (int, error) {
	var err error
	var i int

	vote := entry.Other["vote"]

	temp := strings.Split(vote, "{")[1]
	i, err = strconv.Atoi(strings.Split(temp, ":")[0])
	if err != nil {
		return i, err
	}
	return i, err
}

//check for new round; set HRS, and reset votes if new round
func newStep(status Status, entry LogEntry, numVals int) (Status, error) {
	var err error

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

		status.Proposal = "No"

		status.BlockParts = status.BlockParts[:0]

		status.PreVotes = status.PreVotes[:0]

		status.PreCommits = status.PreCommits[:0]

		for i := 0; i < numVals; i++ {
			status.PreVotes = append(status.PreVotes, "_")
			status.PreCommits = append(status.PreCommits, "_")
		}

	} else if strings.Contains(descrip, "enterPropose") {
		status.Step = "Propose"

	} else if strings.Contains(descrip, "enterPrevote") {
		status.Step = "Prevote"
	} else if strings.Contains(descrip, "enterPrecommit") {
		status.Step = "Precommit"

	} else if strings.Contains(descrip, "enterCommit") {
		status.Step = "Commit"
	}
	return status, err
}

//check for proposal
func checkProp(status Status, entry LogEntry) (Status, error) {
	var err error

	height, err := strconv.Atoi(entry.Other["height"])

	if err != nil {
		return status, err
	}
	if height == status.Height {
		status.Proposal = "yes"
	}

	return status, err
}

//check for block parts
func checkBlock(status Status, entry LogEntry, bpArr []string) (Status, []string, error) {
	var err error
	bpUnique := true
	blockPart := entry.Other["msg"]

	for _, part := range bpArr {
		if part == blockPart {
			bpUnique = false
			break
		}
	}

	if bpUnique == true {
		bpArr = append(bpArr, blockPart)
		status.BlockParts = append(status.BlockParts, "X")
	}
	return status, bpArr, err
}

//add unique votes from validators
func checkVotes(status Status, entry LogEntry, pvArr []string, pcArr []string) (Status, []string, []string, error) {
	var err error

	pvUnique := true
	pcUnique := true

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
				return status, pvArr, pcArr, err
			}

			heightStr := strings.Split(strings.Split(vote, "/")[0], " ")[2]
			roundStr := strings.Split(vote, "/")[1]

			height, err := strconv.Atoi(heightStr)
			if err != nil {
				return status, pvArr, pcArr, err
			}
			round, err := strconv.Atoi(roundStr)
			if err != nil {
				return status, pvArr, pcArr, err
			}

			if height == status.Height && round == status.Round {
				status.PreVotes[i] = "X"
			}
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
				return status, pvArr, pcArr, err
			}

			heightStr := strings.Split(strings.Split(vote, "/")[0], " ")[2]
			roundStr := strings.Split(vote, "/")[1]

			height, err := strconv.Atoi(heightStr)
			if err != nil {
				return status, pvArr, pcArr, err
			}
			round, err := strconv.Atoi(roundStr)
			if err != nil {
				return status, pvArr, pcArr, err
			}

			if height == status.Height && round == status.Round {
				status.PreCommits[i] = "X"
			}
		}
	}
	return status, pvArr, pcArr, err
}

//check for own vote
func myVote(status Status, entry LogEntry, myNode int) (Status, error) {
	var err error

	if status.Step == "Prevote" {
		status.PreVotes[myNode] = "X"
	} else if status.Step == "Precommit" {
		status.PreCommits[myNode] = "X"
	}
	return status, err
}

//*****************************************************************************

//these belong to getMessages
//findPeers reads in a peer file and creates an array of peers
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

//findMyIP parses a verbose log and pulls out the
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

//*****************************************************************************

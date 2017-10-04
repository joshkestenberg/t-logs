package reader

import (
	"bufio"
	"encoding/json"
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
	Level   string
	Date    string
	Time    string
	Descrip string
	Module  string
	Other   map[string]string
}

type Status struct {
	Height      int
	Round       int
	Step        string
	Proposal    string
	BlockParts  []string
	PreVotes    []string
	PreCommits  []string
	XPreVotes   []string
	XPreCommits []string
}

type Peer struct {
	Name   string `json:"name"`
	Ip     string `json:"ip"`
	Pubkey string `json:"pubkey"`
	Index  string `json:"index"`
}

func GetPeers(filenames []string) error {
	var str string
	var err error
	var peers []Peer

	for _, filename := range filenames {
		file, err := os.Open(filename)
		if err != nil {
			return err
		}

		peer := Peer{}

		peer.Name = filename

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if peer.Index != "" && peer.Ip != "" && peer.Name != "" && peer.Pubkey != "" {
				peers = append(peers, peer)
				break
			} else {
				str = scanner.Text()
				entry, err := UnmarshalLine(str)

				if entry.Descrip == "Starting DefaultListener" {
					ip := strings.Split(entry.Other["impl"], "@")[1]
					ip = strings.Split(ip, ":")[0]
					peer.Ip = ip
				} else if strings.Contains(entry.Descrip, "turn to propose") {
					pubkey := strings.Split(entry.Other["privValidator"], "{")[1]
					peer.Pubkey = pubkey[:12]
				} else if peer.Pubkey != "" && strings.Contains(entry.Descrip, "pushed vote") && strings.Contains(entry.Other["vote"], peer.Pubkey) {
					index := strings.Split(entry.Other["vote"], "{")[1]
					peer.Index = strings.Split(index, ":")[0]
				}

				if err = scanner.Err(); err != nil {
					return err
				}
			}
		}
	}

	err = ioutil.WriteFile("nodes.json", nil, 0600)
	if err != nil {
		return err
	}

	file, err := os.OpenFile("nodes.json", os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return err
	}

	err = MarshalJSON(peers, file)

	return err
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

		if strings.Contains(descrip, "Signed proposal block") {
			entry.Descrip = "Signed proposal block: Block{}"
		} else {
			entry.Descrip = "Block{}"
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

func MarshalJSON(peers []Peer, file *os.File) error {
	var err error

	for i := 0; i < len(peers); i++ {
		peer, err := json.Marshal(peers[i])
		if err != nil {
			return err
		}

		_, err = file.WriteString(string(peer) + "\n")
		if err != nil {
			return err
		}
	}

	return err
}

//GetStatus parses log entries for relevant data (see below for all relevant fucntions)
func GetStatus(entries []LogEntry, date string, time string) (Status, error) {
	var err error
	var status Status
	var bpArr []string
	var watch = false

	status.Proposal = "no"

	peers, err := findPeers()
	if err != nil {
		return status, err
	}

	myIP := findMyIP(entries)

	for _, entry := range entries {
		//this logic ensures that we get all entries at a given time rather than just the first
		if entry.Date == date && strings.Contains(entry.Time, time) {
			watch = true
		}

		if watch == true && !strings.Contains(entry.Time, time) {
			break
		}

		if strings.Contains(entry.Descrip, "enter") && !strings.Contains(entry.Descrip, "Invalid") && !strings.Contains(entry.Descrip, "Wait") {
			status, err = newStep(status, entry, peers)
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
			status, err = checkVotes(status, entry, peers)
			if err != nil {
				return status, err
			}
		}

		if entry.Descrip == "Signed and pushed vote" {
			status, err = checkMyVote(status, entry, myIP, peers)
			if err != nil {
				return status, err
			}
		}

		if strings.Contains(entry.Descrip, "Picked") && strings.Contains(entry.Descrip, "Pre") {
			status, err = checkExpected(status, entry, peers, myIP)
			if err != nil {
				return status, err
			}
		}
	}
	return status, err
}

func GetMessages(entries []LogEntry, dur int, stD string, stT string, enD string, enT string, order bool, commits bool) error {
	var trackTime int
	var prevMinute int
	var prevHour int
	var prevMonth int
	var prevDay int
	var parse bool
	var watch bool
	var err error
	var prvTime int

	first := true

	peers, err := findPeers()
	if err != nil {
		return err
	}

	var msgs []string

	myIp := findMyIP(entries)

	for i := 0; i < len(peers); i++ {
		msgs = append(msgs, "_")
	}

	for _, entry := range entries {
		if entry.Date == stD && strings.Contains(entry.Time, stT) && parse == false {
			parse = true
		}

		if entry.Date == enD && strings.Contains(entry.Time, enT) {
			watch = true
		}

		if watch == true && !strings.Contains(entry.Time, enT) {
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
			} else if time < prvTime && order == true {
				fmt.Println(entry.Time, "ERROR: LOGS OUT OF ORDER")
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
				ipParse := strings.Split(peerParse, "{")[2]
				ipParse = strings.Split(ipParse, ":")[0]

				for _, peer := range peers {

					index, err := strconv.Atoi(peer.Index)
					if err != nil {
						return err
					}

					if ipParse == peer.Ip {
						msgs[index] = "X"
						break
					}
				}
			} else if strings.Contains(entry.Descrip, "pushed") || strings.Contains(entry.Descrip, "Picked") {
				for _, peer := range peers {

					index, err := strconv.Atoi(peer.Index)
					if err != nil {
						return err
					}

					if myIp == peer.Ip {
						msgs[index] = "O"
						break
					}
				}
			}

			//processing for block commits
			if entry.Descrip == "Block{}" && commits == true {
				fmt.Println(entry.Time, "Block committed")
			}

			//set prvTime
			prvTime = time
		}
	}
	return err
}

//BELOW FUNCTIONS BELONG TO getStatus ^^

//check for new round; set HRS, and reset votes if new round
func newStep(status Status, entry LogEntry, peers []Peer) (Status, error) {
	var err error

	descrip := entry.Descrip

	if strings.Contains(descrip, "enterNewRound") {
		hrs := strings.Split(descrip, " ")[0]
		heightRound := strings.Split(hrs, "(")[1]
		height := strings.Split(heightRound, "/")[0]
		round := strings.Split(heightRound, "/")[1]
		round = strings.Split(round, ")")[0]

		status.Height, err = strconv.Atoi(height)
		if err != nil {
			return status, err
		}
		status.Round, err = strconv.Atoi(round)
		if err != nil {
			return status, err
		}
		//set new round and reset parameters
		status.Step = "NewRound"

		status.Proposal = "No"

		status.BlockParts = status.BlockParts[:0]

		status.PreVotes = status.PreVotes[:0]
		status.XPreVotes = status.XPreVotes[:0]
		status.PreCommits = status.PreCommits[:0]
		status.XPreCommits = status.XPreCommits[:0]

		for i := 0; i < len(peers); i++ {
			status.PreVotes = append(status.PreVotes, "_")
			status.XPreVotes = append(status.XPreVotes, "_")
			status.PreCommits = append(status.PreCommits, "_")
			status.XPreCommits = append(status.XPreCommits, "_")
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
func checkVotes(status Status, entry LogEntry, peers []Peer) (Status, error) {
	var err error

	vote := entry.Other["msg"]

	if strings.Contains(vote, "Prevote") {

		heightStr := strings.Split(strings.Split(vote, "/")[0], " ")[2]
		roundStr := strings.Split(vote, "/")[1]

		height, err := strconv.Atoi(heightStr)
		if err != nil {
			return status, err
		}

		round, err := strconv.Atoi(roundStr)
		if err != nil {
			return status, err
		}

		if height == status.Height && round == status.Round {
			for _, peer := range peers {

				index, err := strconv.Atoi(peer.Index)
				if err != nil {
					return status, err
				}

				if strings.Contains(entry.Other["msg"], peer.Pubkey) {
					if strings.Contains(entry.Other["msg"], "00000000") {
						status.PreVotes[index] = "Y"
						break
					} else {
						status.PreVotes[index] = "X"
						break
					}
				}
			}
		}
	}

	if strings.Contains(vote, "Precommit") {

		heightStr := strings.Split(strings.Split(vote, "/")[0], " ")[2]
		roundStr := strings.Split(vote, "/")[1]

		height, err := strconv.Atoi(heightStr)
		if err != nil {
			return status, err
		}
		round, err := strconv.Atoi(roundStr)
		if err != nil {
			return status, err
		}

		if height == status.Height && round == status.Round {
			for _, peer := range peers {
				index, err := strconv.Atoi(peer.Index)
				if err != nil {
					return status, err
				}

				if strings.Contains(entry.Other["msg"], peer.Pubkey) {
					if strings.Contains(entry.Other["msg"], "00000000") {
						status.PreCommits[index] = "Y"
						break
					} else {
						status.PreCommits[index] = "X"
						break
					}
				}
			}
		}
	}
	return status, err
}

func checkMyVote(status Status, entry LogEntry, myIP string, peers []Peer) (Status, error) {
	var err error
	voteHeight, err := strconv.Atoi(entry.Other["height"])
	if err != nil {
		log.Fatal(err)
	}

	voteRound, err := strconv.Atoi(entry.Other["round"])
	if err != nil {
		log.Fatal(err)
	}

	for _, peer := range peers {

		index, err := strconv.Atoi(peer.Index)
		if err != nil {
			return status, err
		}

		if peer.Ip == myIP && voteHeight == status.Height && voteRound == status.Round {
			if strings.Contains(entry.Other["vote"], "Prevote") {
				if strings.Contains(entry.Other["vote"], "00000000") {
					status.PreVotes[index] = "N"
					status.XPreVotes[index] = "N"
					break
				} else {
					status.PreVotes[index] = "O"
					status.XPreVotes[index] = "O"
					break
				}
			} else if strings.Contains(entry.Other["vote"], "Precommit") {
				if strings.Contains(entry.Other["vote"], "00000000") {
					status.PreCommits[index] = "N"
					status.XPreCommits[index] = "N"
					break
				} else {
					status.PreCommits[index] = "O"
					status.XPreCommits[index] = "O"
					break
				}
			}
		}
	}

	return status, err
}

func checkExpected(status Status, entry LogEntry, peers []Peer, myIP string) (Status, error) {
	var err error

	for _, peer := range peers {

		index, err := strconv.Atoi(peer.Index)
		if err != nil {
			return status, err
		}

		if strings.Contains(entry.Descrip, "Prevotes") {
			if strings.Contains(entry.Other["peer"], peer.Ip) {
				status.XPreVotes[index] = "X"
				break
			}
		} else if strings.Contains(entry.Descrip, "Precommits") {
			if strings.Contains(entry.Other["peer"], peer.Ip) {
				status.XPreCommits[index] = "X"
				break
			}
		}
	}
	return status, err
}

//*****************************************************************************

//findPeers reads in a peer file and creates an array of peer structs
func findPeers() ([]Peer, error) {
	var peers []Peer
	var peer Peer

	file, err := os.Open("nodes.json")
	if err != nil {
		return peers, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		str := scanner.Text()

		byteStr := []byte(str)

		json.Unmarshal(byteStr, &peer)

		peers = append(peers, peer)
	}

	return peers, err
}

//findMyIP parses a verbose log and pulls out the current node's IP
func findMyIP(entries []LogEntry) string {
	var ip string

	for _, entry := range entries {
		if entry.Descrip == "Starting DefaultListener" {
			ipParse := strings.Split(entry.Other["impl"], "@")[1]
			ip = strings.Replace(ipParse, ")", "", 1)
			ip = strings.Split(ip, ":")[0]
			break
		}
	}
	return ip
}

//*****************************************************************************

package reader

import (
	"fmt"
	"log"
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

type Node struct {
	Name   string `json:"name"`
	Ip     string `json:"ip"`
	Pubkey string `json:"pubkey"`
	Index  string `json:"index"`
}

func (peer *Node) IsComplete() bool {
	if peer.Index != "" && peer.Ip != "" && peer.Name != "" && peer.Pubkey != "" {
		return true
	}
	return false
}

func (peer *Node) Populate(entry LogEntry, name string) {
	if peer.Name != name {
		peer.Name = name
	}

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
}

func (peer *Node) Save(nodes []Node) ([]Node, bool) {
	if peer.IsComplete() {
		nodes = append(nodes, *peer)
		return nodes, true
	} else {
		return nodes, false
	}
}

//GetStatus parses log entries for relevant data (see below for all relevant fucntions)
func GetStatus(entries []LogEntry, nodes []Node, date string, time string) (Status, error) {
	var err error
	var status Status
	var bpArr []string
	var watch = false

	status.Proposal = "no"

	myIP := findMyIP(entries)

	for _, entry := range entries {
		//this logic ensures that we get all entries at a given time rather than just the first
		if entry.Date == date && strings.Contains(entry.Time, time) {
			watch = true
		}

		if watch == true && !strings.Contains(entry.Time, time) {
			break
		}
		//

		if strings.Contains(entry.Descrip, "enter") && !strings.Contains(entry.Descrip, "Invalid") && !strings.Contains(entry.Descrip, "Wait") {
			status, err = newStep(status, entry, nodes)
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
			status, err = checkVotes(status, entry, nodes)
			if err != nil {
				return status, err
			}
		}

		if entry.Descrip == "Signed and pushed vote" {
			status, err = checkMyVote(status, entry, myIP, nodes)
			if err != nil {
				return status, err
			}
		}

		if strings.Contains(entry.Descrip, "Picked") && strings.Contains(entry.Descrip, "Pre") {
			status, err = checkExpected(status, entry, nodes, myIP)
			if err != nil {
				return status, err
			}
		}
	}
	return status, err
}

//*********************************************************************following functions belong to GetStatus***********************************************

//check for new round; set HRS, and reset votes if new round
func newStep(status Status, entry LogEntry, nodes []Node) (Status, error) {
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

		for i := 0; i < len(nodes); i++ {
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
func checkVotes(status Status, entry LogEntry, nodes []Node) (Status, error) {
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
			for _, peer := range nodes {

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
			for _, peer := range nodes {
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

func checkMyVote(status Status, entry LogEntry, myIP string, nodes []Node) (Status, error) {
	var err error
	voteHeight, err := strconv.Atoi(entry.Other["height"])
	if err != nil {
		log.Fatal(err)
	}

	voteRound, err := strconv.Atoi(entry.Other["round"])
	if err != nil {
		log.Fatal(err)
	}

	for _, peer := range nodes {

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

func checkExpected(status Status, entry LogEntry, nodes []Node, myIP string) (Status, error) {
	var err error

	for _, peer := range nodes {

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

//*****************************************************************************end of GetStatus functions*************************************************

func GetMessages(entries []LogEntry, nodes []Node, dur int, stD string, stT string, enD string, enT string, order bool, commits bool) error {
	var trackers = make(map[string]int)
	var prevTime int
	var parse bool
	var watch bool
	var err error

	first := true

	lenNodes := len(nodes)

	var msgs []string

	myIp := findMyIP(entries)

	for i := 0; i < len(nodes); i++ {
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
				trackers = updateParams(trackers, time, minute, hour, day, month)
				first = false
			} else if time < prevTime && order == true {
				//if order flag is added
				fmt.Println(entry.Time, "ERROR: LOGS OUT OF ORDER")
			}

			//this if/else statement determines when to return a statement, accounting for the turnover of minutes, hours, days, months, years
			if (time > trackers["trackTime"] && (time-trackers["trackTime"]) >= dur) || (time < trackers["trackTime"] && (60000-trackers["trackTime"]+time) >= dur) { //duration elapses normally
				trackers, msgs = printAndReset(trackers, msgs, entry, time, minute, hour, day, month, lenNodes)

			} else if (minute == (trackers["prevMinute"]+1) && time > trackers["trackTime"]) || minute >= (trackers["prevMinute"]+2) { //minute rolls over
				trackers, msgs = printAndReset(trackers, msgs, entry, time, minute, hour, day, month, lenNodes)

			} else if hour == (trackers["prevHour"]+1) && ((60000-trackers["trackTime"]+time) >= dur || time > trackers["trackTime"]) { //hour rolls over
				trackers, msgs = printAndReset(trackers, msgs, entry, time, minute, hour, day, month, lenNodes)

			} else if day == (trackers["prevDay"]+1) && ((60000-trackers["trackTime"]+time) >= dur || time > trackers["trackTime"]) { //day rolls over
				trackers, msgs = printAndReset(trackers, msgs, entry, time, minute, hour, day, month, lenNodes)

			} else if (month == (trackers["prevMonth"]+1) || month < trackers["prevMonth"]) && ((60000-trackers["trackTime"]+time) >= dur || time > trackers["trackTime"]) { //month rolls over
				trackers, msgs = printAndReset(trackers, msgs, entry, time, minute, hour, day, month, lenNodes)

			} else if reflect.DeepEqual(entry, entries[len(entries)-1]) { //log ends
				fmt.Println(entry.Time, msgs)
			}

			//processing logic for received msgs

			if entry.Descrip == "Receive" {

				peerParse := strings.Split(entry.Other["src"], "}")[0]
				ipParse := strings.Split(peerParse, "{")[2]
				ipParse = strings.Split(ipParse, ":")[0]

				for _, peer := range nodes {

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
				for _, peer := range nodes {

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

			//set prevTime
			prevTime = time
		}
	}
	return err
}

//*********************************************************************following functions belong to GetMsgs***********************************************

//prints msgs to console, updates time parameters, resets msgs array
func printAndReset(trackers map[string]int, msgs []string, entry LogEntry, time int, minute int, hour int, day int, month int, lenNodes int) (map[string]int, []string) {
	//print to command line
	fmt.Println(entry.Time, msgs)

	//update trackers
	trackers = updateParams(trackers, time, minute, hour, day, month)

	//reset msgs
	for i := 0; i < lenNodes; i++ {
		msgs[i] = "_"
	}

	return trackers, msgs
}

//updates time after checking how much has elapsed
func updateParams(trackers map[string]int, time int, minute int, hour int, day int, month int) map[string]int {
	trackers["trackTime"] = time
	trackers["prevMinute"] = minute
	trackers["prevHour"] = hour
	trackers["prevDay"] = day
	trackers["prevMonth"] = month

	return trackers
}

//*****************************************************************************end of GetStatus functions*************************************************

//*******************************************************************following functions are called in multiple contexts**********************************

//findMyIP finds the current node's IP
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

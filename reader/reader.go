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

func (node *Node) IsComplete() bool {
	if node.Index != "" && node.Ip != "" && node.Name != "" && node.Pubkey != "" {
		return true
	}
	return false
}

func (node *Node) Populate(entry LogEntry, name string) {
	if node.Name != name {
		node.Name = name
	}

	if entry.Descrip == "Starting DefaultListener" {
		ip := strings.Split(entry.Other["impl"], "@")[1]
		ip = strings.Split(ip, ":")[0]
		node.Ip = ip
	} else if strings.Contains(entry.Descrip, "turn to propose") {
		pubkey := strings.Split(entry.Other["privValidator"], "{")[1]
		node.Pubkey = pubkey[:12]
	} else if node.Pubkey != "" && strings.Contains(entry.Descrip, "pushed vote") && strings.Contains(entry.Other["vote"], node.Pubkey) {
		index := strings.Split(entry.Other["vote"], "{")[1]
		node.Index = strings.Split(index, ":")[0]
	}
}

func (node *Node) Save(nodes []Node) ([]Node, bool) {
	if node.IsComplete() {
		nodes = append(nodes, *node)
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

	status.Proposal = "No"

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

		status.BlockParts = []string{}

		status.PreVotes = []string{}
		status.XPreVotes = []string{}
		status.PreCommits = []string{}
		status.XPreCommits = []string{}

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
		status.Proposal = "Yes"
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
			for _, node := range nodes {

				index, err := strconv.Atoi(node.Index)
				if err != nil {
					return status, err
				}

				if strings.Contains(entry.Other["msg"], node.Pubkey) {
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
			for _, node := range nodes {
				index, err := strconv.Atoi(node.Index)
				if err != nil {
					return status, err
				}

				if strings.Contains(entry.Other["msg"], node.Pubkey) {
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

	for _, node := range nodes {

		index, err := strconv.Atoi(node.Index)
		if err != nil {
			return status, err
		}

		if node.Ip == myIP && voteHeight == status.Height && voteRound == status.Round {
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

	for _, node := range nodes {

		index, err := strconv.Atoi(node.Index)
		if err != nil {
			return status, err
		}

		if strings.Contains(entry.Descrip, "Prevotes") {
			if strings.Contains(entry.Other["peer"], node.Ip) {
				status.XPreVotes[index] = "X"
				break
			}
		} else if strings.Contains(entry.Descrip, "Precommits") {
			if strings.Contains(entry.Other["peer"], node.Ip) {
				status.XPreCommits[index] = "X"
				break
			}
		}
	}
	return status, err
}

//*****************************************************************************end of GetStatus functions*************************************************

func GetMessages(entries []LogEntry, nodes []Node, dur int, stD string, stT string, enD string, enT string, order bool, commits bool) ([][]string, error) {
	var trackers = make(map[string]int)
	var timeParams = make(map[string]int)

	var prevTime int
	var parse bool
	var watch bool
	var err error

	var msgArr [][]string

	first := true

	lenNodes := len(nodes)

	var msgs []string

	myIp := findMyIP(entries)

	for i := 0; i < len(nodes); i++ {
		msgs = append(msgs, "_")
	}

	for _, entry := range entries {
		//check if we should start parsing
		if entry.Date == stD && strings.Contains(entry.Time, stT) && parse == false {
			parse = true
		}

		//check if we've reached end time (continue parsing until this time has passed)
		if entry.Date == enD && strings.Contains(entry.Time, enT) {
			watch = true
		}

		if parse == true {
			//parse the entry for time parameters
			timeParams, err = parseEntryTime(timeParams, entry)

			//set up our trackers if first log entry read
			if first == true {
				trackers = updateParams(trackers, timeParams)
				first = false
			} else if timeParams["time"] < prevTime && order == true {
				//if order flag is added
				fmt.Println(entry.Time, "ERROR: LOGS OUT OF ORDER\n")
			}

			//read entry and update msgs
			msgs, err := getMsg(msgs, entry, nodes, myIp, commits)
			if err != nil {
				return msgArr, err
			}

			//return if parse time has elapsed
			if watch == true && !strings.Contains(entry.Time, enT) {
				trackers, msgs, msgArr = printAndReset(trackers, msgs, entry, timeParams, lenNodes, msgArr)
				return msgArr, err
			}

			//this if/else statement determines when to return a statement, accounting for the turnover of minutes, hours, days, months, years
			if normalElapse(timeParams["time"], trackers, dur) {
				trackers, msgs, msgArr = printAndReset(trackers, msgs, entry, timeParams, lenNodes, msgArr)

			} else if nextMinute(timeParams["minute"], trackers, timeParams["time"], dur) {
				trackers, msgs, msgArr = printAndReset(trackers, msgs, entry, timeParams, lenNodes, msgArr)

			} else if nextHour(timeParams["hour"], trackers, timeParams["time"], dur) {
				trackers, msgs, msgArr = printAndReset(trackers, msgs, entry, timeParams, lenNodes, msgArr)

			} else if nextDay(timeParams["day"], trackers, timeParams["time"], dur) {
				trackers, msgs, msgArr = printAndReset(trackers, msgs, entry, timeParams, lenNodes, msgArr)

			} else if nextMonth(timeParams["month"], trackers, timeParams["time"], dur) {
				trackers, msgs, msgArr = printAndReset(trackers, msgs, entry, timeParams, lenNodes, msgArr)

			} else if logEnds(entry, entries) {
				fmt.Println(entry.Time, msgs)
				return msgArr, err
			}

			//reset prevTime to entry time
			prevTime = timeParams["time"]

		}
	}
	return msgArr, err
}

//*********************************************************************following functions belong to GetMsgs***********************************************

func parseEntryTime(timeParams map[string]int, entry LogEntry) (map[string]int, error) {

	var err error

	tParse := strings.Split(entry.Time, ":")
	timeParse := strings.Split(tParse[2], ".")
	mili := timeParse[0] + timeParse[1]

	timeParams["minute"], err = strconv.Atoi(tParse[1])
	if err != nil {
		return timeParams, err
	}
	timeParams["hour"], err = strconv.Atoi(tParse[0])
	if err != nil {
		return timeParams, err
	}

	dateParse := strings.Split(entry.Date, "-")
	timeParams["month"], err = strconv.Atoi(dateParse[0])
	if err != nil {
		return timeParams, err
	}
	timeParams["day"], err = strconv.Atoi(dateParse[1])
	if err != nil {
		return timeParams, err
	}

	timeParams["time"], err = strconv.Atoi(mili)
	if err != nil {
		return timeParams, err
	}

	return timeParams, err
}

//prints msgs to console, updates time parameters, resets msgs array
func printAndReset(trackers map[string]int, msgs []string, entry LogEntry, timeParams map[string]int, lenNodes int, msgArr [][]string) (map[string]int, []string, [][]string) {
	//print to command line
	fmt.Println(entry.Time, msgs)

	//update trackers
	trackers = updateParams(trackers, timeParams)

	//add msgs to array
	msgArr = append(msgArr, msgs)

	//reset msgs
	for i := 0; i < lenNodes; i++ {
		msgs[i] = "_"
	}

	return trackers, msgs, msgArr
}

//updates time after checking how much has elapsed
func updateParams(trackers map[string]int, timeParams map[string]int) map[string]int {
	trackers["trackTime"] = timeParams["time"]
	trackers["prevMinute"] = timeParams["minute"]
	trackers["prevHour"] = timeParams["hour"]
	trackers["prevDay"] = timeParams["day"]
	trackers["prevMonth"] = timeParams["month"]

	return trackers
}

//processing logic for received msgs
func getMsg(msgs []string, entry LogEntry, nodes []Node, myIp string, commits bool) ([]string, error) {
	if entry.Descrip == "Receive" {

		nodeParse := strings.Split(entry.Other["src"], "}")[0]
		ipParse := strings.Split(nodeParse, "{")[2]
		ipParse = strings.Split(ipParse, ":")[0]

		for _, node := range nodes {

			index, err := strconv.Atoi(node.Index)
			if err != nil {
				return msgs, err
			}

			if ipParse == node.Ip {
				msgs[index] = "X"
				break
			}
		}
	} else if strings.Contains(entry.Descrip, "pushed") || strings.Contains(entry.Descrip, "Picked") {
		for _, node := range nodes {

			index, err := strconv.Atoi(node.Index)
			if err != nil {
				return msgs, err
			}

			if myIp == node.Ip {
				msgs[index] = "O"
				break
			}
		}
	} else if entry.Descrip == "Block{}" && commits == true {
		return []string{"Block committed"}, nil
	}

	return msgs, nil

}

func logEnds(entry LogEntry, entries []LogEntry) bool {
	return reflect.DeepEqual(entry, entries[len(entries)-1])
}

func nextMonth(month int, trackers map[string]int, time int, dur int) bool {
	return (month > trackers["prevMonth"] || month < trackers["prevMonth"]) && rolloverElapse(time, trackers, dur)
}

func nextDay(day int, trackers map[string]int, time int, dur int) bool {
	return day > trackers["prevDay"] && rolloverElapse(time, trackers, dur)
}

func nextHour(hour int, trackers map[string]int, time int, dur int) bool {
	return hour > trackers["prevHour"] && rolloverElapse(time, trackers, dur)
}

func nextMinute(minute int, trackers map[string]int, time int, dur int) bool {
	return minute > trackers["prevMinute"] && rolloverElapse(time, trackers, dur)
}

func normalElapse(time int, trackers map[string]int, dur int) bool {
	return time > trackers["trackTime"] && (time-trackers["trackTime"]) >= dur
}

func rolloverElapse(time int, trackers map[string]int, dur int) bool {
	return ((60000-trackers["trackTime"]+time) >= dur || time > trackers["trackTime"])
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

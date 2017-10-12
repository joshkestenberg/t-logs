package reader_test

import (
	"reflect"
	"testing"

	"github.com/joshkestenberg/t-logs/reader"
)

var NODES = []reader.Node{
	reader.Node{"rendered_20170814T043252.000Z_ec2-34-207-122-153.compute-1.amazonaws.com_tendermint.log", "172.31.44.161", "2A3A16F15BEE", "0"},
	reader.Node{"rendered_20170814T043252.000Z_ec2-52-203-239-89.compute-1.amazonaws.com_tendermint.log", "172.31.39.83", "3D3074F7A7D0", "1"},
	reader.Node{"rendered_20170814T043252.000Z_ec2-184-73-145-200.compute-1.amazonaws.com_tendermint.log", "172.31.36.95", "87708B69426D", "2"},
	reader.Node{"rendered_20170814T043252.000Z_ec2-54-210-163-123.compute-1.amazonaws.com_tendermint.log", "172.31.46.4", "B7AACD67CE2E", "3"},
	reader.Node{"rendered_20170814T043252.000Z_ec2-54-152-106-184.compute-1.amazonaws.com_tendermint.log", "172.31.32.72", "E40892926ECF", "4"},
}

func TestIsComplete(t *testing.T) {
	testCases := []struct {
		node     reader.Node
		complete bool
	}{
		{reader.Node{"abc", "1.2.3", "ASDFETG", "4"}, true},
		{reader.Node{"abc", "1.2.3", "", "6"}, false},
		{reader.Node{"", "", "", ""}, false},
	}

	for _, testCase := range testCases {
		if testCase.node.IsComplete() != testCase.complete {
			t.Errorf("%d complete should equal %d", testCase.node, testCase.complete)
		}
	}
}

func TestPopulate(t *testing.T) {
	testCases := []struct {
		name  string
		empty reader.Node
		full  reader.Node
		entry reader.LogEntry
	}{
		{
			"abc", reader.Node{},
			reader.Node{"abc", "172.31.32.72", "", ""}, reader.LogEntry{"", "", "", "Starting DefaultListener", "", map[string]string{"impl": "Listener(@172.31.32.72:46656)"}}},
		{
			"", reader.Node{}, reader.Node{"", "", "E40892926ECF", ""}, reader.LogEntry{"", "", "", "Our turn to propose", "", map[string]string{"privValidator": "PrivValidator{E40892926ECF1071FF28986CAF619AE16E6A6E25 LH:0, LR:0, LS:0}"}}},
		{
			"yo!", reader.Node{"", "", "E40892926ECF", ""}, reader.Node{"yo!", "", "E40892926ECF", "4"}, reader.LogEntry{"", "", "", "Signed and pushed vote", "", map[string]string{"vote": "Vote{4:E40892926ECF 1/00/1(Prevote) 4066B75D9AD4 /17BC2CE1E183.../}"}}},
	}

	for _, testCase := range testCases {
		testCase.empty.Populate(testCase.entry, testCase.name)
		if testCase.empty != testCase.full {
			t.Errorf("%d should equal %d", testCase.empty, testCase.full)
		}
	}
}

func TestSave(t *testing.T) {
	var nodes []reader.Node
	var val bool
	var saved = false

	testCases := []struct {
		node     reader.Node
		complete bool
	}{
		{reader.Node{"abc", "1.2.3", "ASDFETG", "4"}, true},
		{reader.Node{"abc", "1.2.3", "", "6"}, false},
		{reader.Node{"", "", "", ""}, false},
	}

	for _, testCase := range testCases {
		nodes, val = testCase.node.Save(nodes)

		if testCase.complete && val == true {

			for _, node := range nodes {
				if node == testCase.node {
					saved = true
				}
			}

			if !saved {
				t.Error("Node unsaved in array but should be")
			}
		} else {
			for _, node := range nodes {
				if node == testCase.node {
					t.Error("Node saved in array but should not be")
				}
			}
		}
	}
}

func TestGetStatus(t *testing.T) {
	nodes := NODES

	testCases := []struct {
		entries []reader.LogEntry
		status  reader.Status
	}{
		{
			//ensure newRound resets array and parses height/round; ignores invalid args entry
			[]reader.LogEntry{
				reader.LogEntry{"", "08-14", "0", "Starting DefaultListener", "", map[string]string{"impl": "Listener(@172.31.32.72:46656)"}},
				reader.LogEntry{"", "08-14", "1", "enterPrecommit(7/0): Invalid args. Current step: 7/0/RoundStepPrecommit", "", map[string]string{}},
				reader.LogEntry{"", "08-14", "1", "enterNewRound(8/0). Current: 8/0/RoundStepNewHeight", "", map[string]string{}},
			},
			reader.Status{
				8, 0, "NewRound", "No", []string{}, []string{"_", "_", "_", "_", "_"}, []string{"_", "_", "_", "_", "_"}, []string{"_", "_", "_", "_", "_"}, []string{"_", "_", "_", "_", "_"},
			},
		},
		{
			//ensure enterPropose, checkProp, checkBlock functional (pass same BlockPart twice to make sure it only shows up once)
			[]reader.LogEntry{
				reader.LogEntry{"", "08-14", "0", "Starting DefaultListener", "", map[string]string{"impl": "Listener(@172.31.32.72:46656)"}},
				reader.LogEntry{"", "08-14", "1", "enterNewRound(1/0). Current: 1/0/RoundStepNewHeight", "", map[string]string{}},
				reader.LogEntry{"", "08-14", "1", "enterPropose(1/0). Current: 1/0/RoundStepNewRound", "consensus", map[string]string{}},
				reader.LogEntry{"", "08-14", "1", "Received complete proposal block", "consensus", map[string]string{"height": "1"}},
				reader.LogEntry{"", "08-14", "1", "Receive", "consensus", map[string]string{"msg": "[BlockPart H:1 R:0 P:Part{#0\n  Bytes: 010101066A65...\n  Proof: SimpleProof{\n    Aunts: []\n  }\n}]"}},
				reader.LogEntry{"", "08-14", "1", "Receive", "consensus", map[string]string{"msg": "[BlockPart H:1 R:0 P:Part{#0\n  Bytes: 010101066A65...\n  Proof: SimpleProof{\n    Aunts: []\n  }\n}]"}},
			},
			reader.Status{
				1, 0, "Propose", "Yes", []string{"X"}, []string{"_", "_", "_", "_", "_"}, []string{"_", "_", "_", "_", "_"}, []string{"_", "_", "_", "_", "_"}, []string{"_", "_", "_", "_", "_"},
			},
		},
		{
			//ensure checkVotes is functional
			[]reader.LogEntry{
				reader.LogEntry{"", "08-14", "0", "Starting DefaultListener", "", map[string]string{"impl": "Listener(@172.31.32.72:46656)"}},
				reader.LogEntry{"", "08-14", "1", "enterNewRound(1/0). Current: 1/0/RoundStepNewHeight", "", map[string]string{}},
				reader.LogEntry{"", "08-14", "1", "Received complete proposal block", "consensus", map[string]string{"height": "1"}},
				reader.LogEntry{"", "08-14", "1", "enterPrevote(1/0). Current: 1/0/RoundStepPropose", "consensus", map[string]string{}},
				reader.LogEntry{"", "08-14", "1", "Receive", "",
					map[string]string{"msg": "[Vote Vote{1:3D3074F7A7D0 1/00/1(Prevote) 4066B75D9AD4 /5FC6C5515A66.../}]"}},
			},
			reader.Status{
				1, 0, "Prevote", "Yes", []string{}, []string{"_", "X", "_", "_", "_"}, []string{"_", "_", "_", "_", "_"}, []string{"_", "_", "_", "_", "_"}, []string{"_", "_", "_", "_", "_"},
			},
		},
		{
			//checkVotes is dynamic and checkMyVote is functional
			[]reader.LogEntry{
				reader.LogEntry{"", "08-14", "0", "Starting DefaultListener", "", map[string]string{"impl": "Listener(@172.31.32.72:46656)"}},
				reader.LogEntry{"", "08-14", "1", "enterNewRound(1/0). Current: 1/0/RoundStepNewHeight", "", map[string]string{}},
				reader.LogEntry{"", "08-14", "1", "Signed and pushed vote", "",
					map[string]string{"height": "1", "round": "0", "vote": "Vote{4:E40892926ECF 1/00/2(Prevote) 4066B75D9AD4 /C28769ADBAD2.../}"}},
				reader.LogEntry{"", "08-14", "1", "enterPrecommit(1/0). Current: 1/0/RoundStepPrevote", "consensus", map[string]string{}},
				reader.LogEntry{"", "08-14", "1", "Receive", "",
					map[string]string{"msg": "[Vote Vote{1:3D3074F7A7D0 1/00/1(Prevote) 4066B75D9AD4 /5FC6C5515A66.../}]"}},
				reader.LogEntry{"", "08-14", "1", "Receive", "",
					map[string]string{"msg": "[Vote Vote{0:2A3A16F15BEE 1/00/1(Prevote) 4066B75D9AD4 /202CFFFBC76C.../}]"}},
				reader.LogEntry{"", "08-14", "1", "Receive", "",
					map[string]string{"msg": "[Vote Vote{0:2A3A16F15BEE 1/00/2(Precommit) 4066B75D9AD4 /E0B6722E7670.../}]"}},
				reader.LogEntry{"", "08-14", "1", "Signed and pushed vote", "",
					map[string]string{"height": "1", "round": "0", "vote": "Vote{4:E40892926ECF 1/00/2(Precommit) 4066B75D9AD4 /C28769ADBAD2.../}"}},
			},
			reader.Status{
				1, 0, "Precommit", "No", []string{}, []string{"X", "X", "_", "_", "O"}, []string{"X", "_", "_", "_", "O"}, []string{"_", "_", "_", "_", "O"}, []string{"_", "_", "_", "_", "O"},
			},
		},
		{
			//checkExpected is functional
			[]reader.LogEntry{
				reader.LogEntry{"", "08-14", "0", "Starting DefaultListener", "", map[string]string{"impl": "Listener(@172.31.32.72:46656)"}},
				reader.LogEntry{"", "08-14", "1", "enterNewRound(1/0). Current: 1/0/RoundStepNewHeight", "", map[string]string{}},
				reader.LogEntry{"", "08-14", "1", "enterPrecommit(1/0). Current: 1/0/RoundStepPrevote", "consensus", map[string]string{}},
				reader.LogEntry{"", "08-14", "1", "Receive", "",
					map[string]string{"msg": "[Vote Vote{1:3D3074F7A7D0 1/00/1(Prevote) 4066B75D9AD4 /5FC6C5515A66.../}]"}},
				reader.LogEntry{"", "08-14", "1", "Receive", "",
					map[string]string{"msg": "[Vote Vote{0:2A3A16F15BEE 1/00/1(Prevote) 000000000000 /202CFFFBC76C.../}]"}},
				reader.LogEntry{"", "08-14", "1", "Receive", "",
					map[string]string{"msg": "[Vote Vote{0:2A3A16F15BEE 1/00/2(Precommit) 4066B75D9AD4 /E0B6722E7670.../}]"}},
				reader.LogEntry{"", "08-14", "1", "Signed and pushed vote", "",
					map[string]string{"height": "1", "round": "0", "vote": "Vote{4:E40892926ECF 1/00/2(Prevote) 000000000000 /C28769ADBAD2.../}"}},
				reader.LogEntry{"", "08-14", "1", "Signed and pushed vote", "",
					map[string]string{"height": "1", "round": "0", "vote": "Vote{4:E40892926ECF 1/00/2(Precommit) 000000000000 /C28769ADBAD2.../}"}},
				reader.LogEntry{"", "08-14", "1", "enterCommit(1/0). Current: 1/0/RoundStepPrecommit", "consensus", map[string]string{}},
				reader.LogEntry{"", "08-14", "1", "Picked rs.Prevotes(prs.Round) to send", "consensus", map[string]string{"peer": "172.31.46.4"}},
				reader.LogEntry{"", "08-14", "1", "Picked rs.Precommits(prs.Round) to send", "consensus", map[string]string{"peer": "172.31.46.4"}},
			},
			reader.Status{
				1, 0, "Commit", "No", []string{}, []string{"Y", "X", "_", "_", "N"}, []string{"X", "_", "_", "_", "N"}, []string{"_", "_", "_", "X", "N"}, []string{"_", "_", "_", "X", "N"},
			},
		},
	}
	for _, testCase := range testCases {
		status, _ := reader.GetStatus(testCase.entries, nodes, "08-14", "1")
		if !reflect.DeepEqual(testCase.status, status) {
			t.Errorf("expected %s, received %s", testCase.status, status)
		}
	}
}

func TestGetMessages(t *testing.T) {

	nodes := NODES

	testCases := []struct {
		entries []reader.LogEntry
		msgArr  [][]string
	}{
		{
			//assert all nodes register accross different times
			[]reader.LogEntry{
				reader.LogEntry{"", "08-14", "00:00:00.000", "Starting DefaultListener", "", map[string]string{"impl": "Listener(@172.31.36.95:46656)"}},
				reader.LogEntry{"", "08-14", "00:00:00.001", "Receive", "",
					map[string]string{"src": "Peer{MConn{172.31.44.161:46656} 4F85D81F23AB out}"}},
				reader.LogEntry{"", "08-14", "00:00:00.002", "Receive", "",
					map[string]string{"src": "Peer{MConn{172.31.39.83:46656} 1BF7CA4820CD out}"}},
				reader.LogEntry{"", "08-14", "00:00:00.003", "Signed and pushed vote", "",
					map[string]string{"height": "1", "round": "0", "vote": "Vote{4:E40892926ECF 1/00/2(Precommit) 000000000000 /C28769ADBAD2.../}"}},
				reader.LogEntry{"", "08-14", "00:00:00.004", "Receive", "",
					map[string]string{"src": "Peer{MConn{172.31.46.4:58661} 1BF7CA4820CD out}"}},
				reader.LogEntry{"", "08-14", "00:00:00.005", "Receive", "",
					map[string]string{"src": "Peer{MConn{172.31.32.72:46656} D7ECACFDC10F in}"}},
			},
			[][]string{
				{"X", "_", "_", "_", "_"}, {"_", "X", "_", "_", "_"}, {"_", "_", "O", "_", "_"}, {"_", "_", "_", "X", "_"}, {"_", "_", "_", "_", "X"},
			},
		},
		// {
		// 	[]reader.LogEntry{},
		// 	[][]string{
		// 		{}, {}, {},
		// 	},
		// },
	}

	for _, testCase := range testCases {

		msgArr, _ := reader.GetMessages(testCase.entries, nodes, 1, "08-14", "00:00:00.000", "08-14", "00:00:00.005", false, false)
		if !reflect.DeepEqual(testCase.msgArr, msgArr) {
			t.Errorf("expected %s, received %s", testCase.msgArr, msgArr)
		}
	}

}

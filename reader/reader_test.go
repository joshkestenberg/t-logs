package reader_test

import (
	"testing"

	"github.com/joshkestenberg/t-logs/reader"
)

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

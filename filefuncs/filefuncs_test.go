package filefuncs_test

import (
	"reflect"
	"testing"

	"github.com/joshkestenberg/t-logs/filefuncs"
	"github.com/joshkestenberg/t-logs/reader"
)

func TestUnmarshalLine(t *testing.T) {
	testCases := []struct {
		line string
		exp  reader.LogEntry
	}{
		{
			//random parse example
			"I[08-14|04:33:04.650] Ignoring updateToState()                     module=consensus newHeight=1 oldHeight=1",
			reader.LogEntry{
				"I", "08-14", "04:33:04.650", "Ignoring updateToState()", "consensus", map[string]string{"newHeight": "1", "oldHeight": "1"},
			},
		},
		{
			//block parse example
			"D[08-14|04:33:04.652] Signed proposal block: Block{}",
			reader.LogEntry{
				"D", "08-14", "04:33:04.652", "Signed proposal block: Block{}", "consensus", map[string]string{},
			},
		},
	}

	for _, testCase := range testCases {
		rec, _ := filefuncs.UnmarshalLine(testCase.line)

		if !reflect.DeepEqual(testCase.exp, rec) {
			t.Errorf("expected %f, received %f", testCase.exp, rec)
		}
	}
}

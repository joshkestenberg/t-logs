package main

import (
	"bufio"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestOpenLog(t *testing.T) {
	//test os.Open
	file, err := OpenLog("./tendermint.log")
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	//test correct file name
	f, err := file.Stat()
	if err != nil {
		t.Log(err)
		t.Fail()
	} else if f.Name() != "tendermint.log" {
		t.Log(f.Name())
		t.Fail()
	}
}

func TestInitJSON(t *testing.T) {
	//break ioutil.WriteFile with invalid filename
	faultyFile, err := InitJSON("ill/egal")
	if err == nil {
		t.Log(faultyFile, err)
		t.Fail()
	}

	//test os.OpenFile returns no error
	jsonFile, err := InitJSON("hello.json")
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	//test correct file name
	jsonF, err := jsonFile.Stat()
	if err != nil {
		t.Log(err)
		t.Fail()
	} else if jsonF.Name() != "hello.json" {
		t.Log("expected:  hello.json, got: " + jsonF.Name())
		t.Fail()
	}
}

func TestRenderDoc(t *testing.T) {
	file, _ := OpenLog("./tendermint.log")
	render, err := RenderDoc(file, 1, 10)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	//check correct filename
	f, err := render.Stat()
	if err != nil {
		t.Log(err)
		t.Fail()
	} else if f.Name() != "rendered_tendermint.log" {
		t.Log("expected filename 'rendered_Tendermint.log', received: ", f.Name())
		t.Fail()
	}

	//check for invalid lines
	scanner := bufio.NewScanner(render)
	for scanner.Scan() {
		str := scanner.Text()
		if !strings.Contains(str, `|`) {
			t.Log("invalid string present in render: ", str)
			t.Fail()
		}
	}

}

func TestUnmarshalLine(t *testing.T) {
	type testCase struct {
		logLine string
		entry   LogEntry
	}

	casesContent := []testCase{
		{
			logLine: `I[today|9] Some msg    module=stuff this=yea that=no`,
			entry: LogEntry{
				Level:   "I",
				Date:    "today",
				Time:    "9",
				Descrip: "Some msg",
				Module:  "stuff",
				Other:   map[string]string{`this`: `yea`, `that`: `no`},
			},
		},
		{
			logLine: `D[07-06|19:56:13.196] addVote                                      module=consensus voteHeight=8 voteType=1 csHeight=8`,
			entry: LogEntry{
				Level:   "D",
				Date:    "07-06",
				Time:    "19:56:13.196",
				Descrip: "addVote",
				Module:  "consensus",
				Other:   map[string]string{`voteHeight`: `8`, `voteType`: `1`, `csHeight`: `8`},
			},
		},
	}

	for _, c := range casesContent[0:2] {
		entry, err := UnmarshalLine(c.logLine)
		eq := reflect.DeepEqual(entry, c.entry)
		if !eq {
			fmt.Println(entry.Other)
			t.Log("return data: ", entry, " test data: ", c.entry)
			t.Fail()
		} else if err != nil {
			t.Log(err)
			t.Fail()
		}
	}
}

func TestUnmarshalLines(t *testing.T) {
	// TODO: test correct line number renders correct output
}

func TestMarshalJSON(t *testing.T) {
	type testCase struct {
		json  string
		entry LogEntry
	}

	var entries []LogEntry

	file, _ := InitJSON("hello.json")

	cases := []testCase{
		{
			json: `{"Level":"I","Date":"today","Time":"9","Descrip":"Some msg","Module":"stuff","other":{"this":"yea","that":"no"}}`,
			entry: LogEntry{
				Level:   "I",
				Date:    "today",
				Time:    "9",
				Descrip: "Some msg",
				Module:  "stuff",
				Other:   map[string]string{`this`: `yea`, `that`: `no`},
			},
		},
		{
			json: `{"level":"D","date":"07-06","time":"19:56:13.196","descrip":"addVote","module":"consensus","other":{"voteHeight":"8","voteType":"1","csHeight":"8"}}`,
			entry: LogEntry{
				Level:   "D",
				Date:    "07-06",
				Time:    "19:56:13.196",
				Descrip: "addVote",
				Module:  "consensus",
				Other:   map[string]string{`voteHeight`: `8`, `voteType`: `1`, `csHeight`: `8`},
			},
		},
	}

	for _, c := range cases {
		entries = append(entries, c.entry)
	}

	err := MarshalJSON(entries, file)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		str := scanner.Text()
		if str != cases[count].json {
			t.Log("return data: ", str, " test data: ", cases[count].json)
			t.Fail()
		}
		count++
	}

}

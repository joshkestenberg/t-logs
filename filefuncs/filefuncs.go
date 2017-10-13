package filefuncs

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"

	"github.com/joshkestenberg/t-logs/reader"
)

//OpenLog opens log file
func OpenLog(filepath string) (*os.File, error) {
	return os.Open(filepath)
}

//RenderDoc removes blocks and spaces to prep doc for parse
func RenderDoc(file *os.File, name string) (string, error) {
	var err error

	renderName := "rendered"

	splitName := strings.Split(name, `/`)

	for _, word := range splitName {
		renderName = renderName + "_" + word
	}

	if _, err := os.Stat("./" + renderName); err == nil {
		return renderName, err
	}

	err = ioutil.WriteFile(renderName, nil, 0600)
	if err != nil {
		return renderName, err
	}

	renderFile, err := os.OpenFile(renderName, os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return renderName, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		str := scanner.Text()
		if strings.Contains(str, `|`) {
			if strings.Contains(str, "Block{") {
				str = str + "}"
			}
			_, err = renderFile.WriteString(str + "\n")
			if err != nil {
				return renderName, err
			}
		}
	}

	return renderName, err
}

//UnmarshalLines converts given lines to an array of structs
func UnmarshalLines(file *os.File) ([]reader.LogEntry, error) {
	var str string
	var err error
	var entries []reader.LogEntry

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

//UnmarshalLine takes one line at a time and outputs a struct
func UnmarshalLine(line string) (reader.LogEntry, error) {
	var entry reader.LogEntry
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
		entry.Other = map[string]string{}

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

//populates Nodes and writes to json file
func GetNodes(filenames []string) error {
	var str string
	var err error
	var saved bool
	var nodes []reader.Node

	for _, filename := range filenames {

		file, err := os.Open(filename)

		if err != nil {
			return err
		}

		node := reader.Node{}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			str = scanner.Text()
			entry, err := UnmarshalLine(str)

			nodes, saved = node.Save(nodes)
			if saved == true {
				break
			} else {
				node.Populate(entry, filename)
			}

			if err = scanner.Err(); err != nil {
				return err
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

	err = MarshalJSON(nodes, file)

	return err
}

//takes populated Node and writes to json file
func MarshalJSON(nodes []reader.Node, file *os.File) error {
	var err error

	for i := 0; i < len(nodes); i++ {
		node, err := json.Marshal(nodes[i])
		if err != nil {
			return err
		}

		_, err = file.WriteString(string(node) + "\n")
		if err != nil {
			return err
		}
	}

	return err
}

//unmarshalNodes unmarshalls nodes.json to []Node
func UnmarshalNodes() ([]reader.Node, error) {
	var nodes []reader.Node
	var node reader.Node

	file, err := os.Open("nodes.json")
	if err != nil {
		return nodes, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		str := scanner.Text()

		byteStr := []byte(str)

		json.Unmarshal(byteStr, &node)

		nodes = append(nodes, node)
	}

	return nodes, err
}

package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type WebStruct struct {
	Height     int
	Round      int
	Step       string
	Proposal   string
	BlockParts string
	PreVotes   string
	PreCommits string
}

var status Status
var blockParts string
var preVotes string
var preCommits string
var WS WebStruct

func newHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("./form.html")
	t.Execute(w, nil)
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(100)

	_, handler, err := r.FormFile("upload")
	if err != nil {
		log.Fatal(err)
	}

	ln, err := strconv.Atoi(r.Form["line"][0])
	if err != nil {
		log.Fatal(err)
	}

	logFile, err := OpenLog(handler.Filename)
	if err != nil {
		log.Fatal(err)
	}

	renderFile, err := RenderDoc(logFile, ln)
	if err != nil {
		log.Fatal(err)
	}

	entries, err := UnmarshalLines(renderFile)
	if err != nil {
		log.Fatal(err)
	}

	status, err = GetStatus(entries)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(status)

	http.Redirect(w, r, "/data", 301)
}

func displayHandler(w http.ResponseWriter, r *http.Request) {
	for _, bp := range status.BlockParts {
		blockParts += bp
	}

	for _, pv := range status.PreVotes {
		preVotes += pv
	}
	for _, pc := range status.PreCommits {
		preCommits += pc
	}

	WS.Height = status.Height
	WS.Round = status.Round
	WS.Step = status.Step
	WS.Proposal = status.Proposal
	WS.BlockParts = blockParts
	WS.PreVotes = preVotes
	WS.PreCommits = preCommits

	t, err := template.ParseFiles("./display.html")
	if err != nil {
		log.Fatal(err)
	}

	t.Execute(w, WS)

}

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/", newHandler).Methods("GET")
	r.HandleFunc("/data", dataHandler).Methods("POST")
	r.HandleFunc("/data", displayHandler).Methods("GET")

	s := http.StripPrefix("/resources/", http.FileServer(http.Dir("./resources/")))
	r.PathPrefix("/resources/").Handler(s)
	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":8000", r))
}

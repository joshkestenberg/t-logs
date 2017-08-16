package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
)

func newHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>Upload</h1><br><p>Upload your log file and enter the line number at which you which to inquire about the state of the node.<p><br><form enctype='multipart/form-data' action='/data' method='POST'><input type='file' name='upload'><br><input type='number' name='line'><br><input type='submit'></form>")
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

	status, err := getStatus(entries)
	if err != nil {
		log.Fatal(err)
	}
}

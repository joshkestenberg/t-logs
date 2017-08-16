package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/", newHandler).Methods("GET")
	r.HandleFunc("/data", dataHandler).Methods("POST")
	log.Fatal(http.ListenAndServe(":8000", r))
}

package main

import (
	"log"
	"net/http"
	"os"

	"keepersecurity.com/ksm-scim/internal/runner"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", syncHandler)
	mux.HandleFunc("/health", healthHandler)

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func syncHandler(w http.ResponseWriter, r *http.Request) {
	syncStat, err := runner.RunFromEnv()
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	runner.PrintStatistics(w, syncStat)
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

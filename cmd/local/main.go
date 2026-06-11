package main

import (
	"errors"
	"log"
	"os"
	"path"
	"strings"

	"keepersecurity.com/ksm-scim/internal/runner"
)

func main() {
	var filePath = "config.base64"
	var err error
	if _, err = os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		filePath = path.Join(homeDir, filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal(err)
	}
	var configData = string(data)
	configData = strings.TrimRight(configData, "\n")

	var recordUID string
	if len(os.Args) == 2 {
		recordUID = os.Args[1]
	}

	syncStat, err := runner.RunFromConfig(configData, recordUID)
	if err != nil {
		log.Fatal(err)
	}

	runner.PrintStatistics(os.Stdout, syncStat)
}

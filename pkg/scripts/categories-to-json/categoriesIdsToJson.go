package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/grafana/auto-triage/pkg/github"
)

var (
	categoriesIdsFile = flag.String(
		"categoriesIdsFile",
		"fixtures/categorizedIds.txt",
		"File containing the ids of the categories to process. One per line",
	)
	outFilePath = flag.String("outFile", "fixtures/categorizedIds.json", "Output file")
)

var githubToken = os.Getenv("GH_TOKEN")

type Issue struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"body"`
	Labels      []string `json:"labels"`
}

func main() {
	flag.Parse()

	if githubToken == "" {
		log.Fatal("GH_TOKEN env var is required")
	}

	// read the file
	file, err := os.Open(*categoriesIdsFile)
	if err != nil {
		log.Fatal("Error opening file:", err)
	}
	defer file.Close()

	// create a new scanner
	scanner := bufio.NewScanner(file)

	var lastRequestTime time.Time = time.Now()

	finalIssues := []Issue{}

	// iterate over the scanner
	for scanner.Scan() {
		// read the line
		line := scanner.Text()

		if line == "" {
			continue
		}

		issueId, err := strconv.Atoi(line)
		if err != nil {
			fmt.Printf("Error converting line to int: %v\n", err)
			continue
		}

		fmt.Printf("  :: Processing issue %d\n", issueId)

		// throttle requests to github to prevent rate limiting
		timeTo1Second := time.Until(lastRequestTime.Add(time.Second * 1))
		if timeTo1Second > 0 {
			fmt.Printf("     -> Sleeping for %v\n", timeTo1Second)
			time.Sleep(timeTo1Second)
		}
		lastRequestTime = time.Now()

		issue, err := github.FetchIssueDetails(issueId, "grafana/grafana")
		if err != nil {
			fmt.Printf("Error fetching issue details: %v\n", err)
			continue
		}

		fmt.Printf("  :: Issue ID: %d\n", issueId)

		title := issue.Title
		description := issue.Body
		labels := make([]string, len(issue.Labels))
		for i, label := range issue.Labels {
			labels[i] = label.Name
		}

		finalIssues = append(
			finalIssues,
			Issue{
				ID:          issueId,
				Title:       title,
				Description: description,
				Labels:      labels,
			},
		)
	}

	jsonBytes, err := json.Marshal(finalIssues)
	if err != nil {
		log.Fatal("Error marshalling issues:", err)
	}

	err = os.WriteFile(*outFilePath, jsonBytes, 0644)
	if err != nil {
		log.Fatal("Error writing to file:", err)
	}

	fmt.Printf("Done writing to %s\n", *outFilePath)
}

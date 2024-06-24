package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/auto-triage/pkg/commontypes"
	_ "github.com/mattn/go-sqlite3"
)

var (
	dbFileName = "github-data.sqlite"
	apiURL     = "https://api.github.com/repos/grafana/grafana/issues"
)

func main() {
	db, err := sql.Open("sqlite3", dbFileName)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	err = createTable(db)
	if err != nil {
		panic(err)
	}
	startPage := getStartPage()
	fmt.Printf("Starting from page %d\n", startPage)
	scrapeIssues(db, startPage)
}

func createTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS issues (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			processed BOOLEAN NOT NULL DEFAULT 0,
			description TEXT,
			labels TEXT,
			raw TEXT NOT NULL
		)
	`)
	return err
}

func getStartPage() int {
	var page int
	pageFile, err := os.ReadFile(".page.txt")
	if err != nil {
		fmt.Printf("Error reading .page.txt: %v\n", err)
	}
	page, err = strconv.Atoi(strings.Trim(string(pageFile), "\n"))
	if err != nil {
		fmt.Printf("Error parsing .page.txt: %v\n", err)
	}
	fmt.Printf("Starting from page %d\n", page)
	return page
}

func setStartPage(page int) {
	err := os.WriteFile(".page.txt", []byte(fmt.Sprintf("%d\n", page)), 0644)
	if err != nil {
		fmt.Printf("Error writing .page.txt: %v\n", err)
	}
}

func scrapeIssues(db *sql.DB, startPage int) {
	var timeout int
	for page := startPage; ; page++ {
		fmt.Printf("Fetching page %d...\n", page)
		apiWithPage := fmt.Sprintf(
			"%s?page=%d&per_page=100&state=closed&sort=created&direction=asc",
			apiURL,
			page,
		)
		req, err := http.NewRequest("GET", apiWithPage, nil)
		if err != nil {
			log.Fatalf("failed to create request: %v", err)
		}

		req.Header.Set("Authorization", "token "+os.Getenv("GH_TOKEN"))

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			fmt.Printf("Error getting url %s: %s\n", apiWithPage, resp.Status)
			log.Fatalf("failed to get response: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("failed to read response body: %v", err)
		}

		var issues []commontypes.Issue
		err = json.Unmarshal(body, &issues)
		if err != nil {
			log.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(issues) == 0 {
			log.Println("No more issues to process. Exiting...")
			break
		}

		for _, issue := range issues {
			issue.Raw = json.RawMessage(body)
			if issue.PullRequest.URL != "" {
				fmt.Println("  - Skipping pull request:", issue.Number)
				continue
			}

			fmt.Println("  - Saving issue:", issue.Number)
			saveIssue(db, issue)
		}

		//set timeout between 1 to 5 random
		timeout = rand.Intn(3) + 1
		setStartPage(page + 1)
		fmt.Println("Sleeping for", timeout, "seconds...")
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func saveIssue(db *sql.DB, issue commontypes.Issue) {
	labels, err := json.Marshal(issue.Labels)
	if err != nil {
		log.Fatalf("failed to marshal labels: %v", err)
	}

	_, err = db.Exec(`
		INSERT OR REPLACE INTO issues (id, title, description, processed, labels, raw)
		VALUES (?, ?, ?, ?, ?)
	`, issue.Number, issue.Title, issue.Description, 0, string(labels), string(issue.Raw))

	if err != nil {
		log.Fatalf("failed to insert issue: %v", err)
	}
}

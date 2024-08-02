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
	startPage := getStartPage(db)
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
	if err != nil {
		return err
	}

	// create table page index
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS page_index (
			page INTEGER PRIMARY KEY,
			last_updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	return nil
}

func getStartPage(db *sql.DB) int {
	// get last page from page index
	var page int

	row := db.QueryRow("SELECT page FROM page_index ORDER BY last_updated DESC LIMIT 1")
	err := row.Scan(&page)
	if err != nil {
		if err == sql.ErrNoRows {
			// no rows found, start from page 1
			return 1
		}
		log.Fatalf("Error getting last page from page_index: %v", err)
	}

	return page
}

func setStartPage(page int, db *sql.DB) {
	_, err := db.Exec("INSERT INTO page_index (page) VALUES (?)", page)
	if err != nil {
		log.Fatalf("Error updating page_index: %v", err)
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
			rawB, err := json.Marshal(issue)
			if err != nil {
				log.Fatalf("failed to marshal raw: %v", err)
				issue.Raw = ""
			}
			issue.Raw = string(rawB)
			if issue.PullRequest.URL != "" {
				fmt.Println("  - Skipping pull request:", issue.Number)
				continue
			}

			fmt.Println("  - Saving issue:", issue.Number)
			saveIssue(db, issue)
		}

		//set timeout between 1 to 3 random
		timeout = rand.Intn(3) + 1
		setStartPage(page+1, db)
		fmt.Println("Sleeping for", timeout, "seconds...")
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}

func saveIssue(db *sql.DB, issue commontypes.Issue) {
	labels := labelsToJSONArray(issue.Labels)

	// check if issue already exists in db
	row := db.QueryRow(`SELECT id FROM issues WHERE id = ?`, issue.Number)
	var id int
	err := row.Scan(&id)
	if err == nil {
		fmt.Println("     -> Issue already exists in db")
		// return
	}

	_, err = db.Exec(`
		INSERT OR REPLACE INTO issues (id, title, description, processed, labels, raw)
		VALUES (?, ?, ?, ?, ?, ?)
	`, issue.Number, issue.Title, issue.Description, 0, string(labels), string(issue.Raw))

	if err != nil {
		log.Fatalf("failed to insert issue: %v", err)
	}

	fmt.Println("     -> Inserted issue into db")
}

func labelsToJSONArray(labels []commontypes.Label) string {
	var jsonArray []string
	for _, label := range labels {
		jsonArray = append(jsonArray, fmt.Sprintf(`"%s"`, label.Name))
	}
	return fmt.Sprintf("[%s]", strings.Join(jsonArray, ","))
}

package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/auto-triage/pkg/github"
	_ "github.com/mattn/go-sqlite3"

	"flag"
	"fmt"
	"os"
)

type PromptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type PromptTemplate struct {
	Messages []PromptMessage `json:"messages"`
}

var (
	issueDbFile        = flag.String("issuesDb", "github-data.sqlite", "Issue database file")
	categorizedIdsFile = flag.String(
		"categorizedIdsFile",
		"",
		"File containing the ids of the issues to process. One per line",
	)
	categorizableIdsFile = flag.String(
		"categorizableIdsFile",
		"",
		"File containing the ids of the issues that are consideredd categorizable. One per line",
	)
	missingInfoIdsFile = flag.String(
		"missingInfoIdsFile",
		"",
		"File containing the ids of the issues that are missing information. One per line",
	)
	openAiKey  = os.Getenv("OPENAI_API_KEY")
	labelsFile = flag.String(
		"labelsFile",
		"fixtures/areaLabels.txt",
		"Labels file. One label per line",
	)
	typesFile = flag.String(
		"typesFile",
		"fixtures/typeLabels.txt",
		"Types file. One label per line",
	)
	outFile = flag.String("outFile", "fine-tune-dataset.json", "Output file")
)

var maxTokens = 300000

func main() {
	var err error
	flag.Parse()

	err = validateFlags()
	if err != nil {
		fmt.Printf("Error validating flags: %v\n", err)
		os.Exit(1)
	}

	db, err := getDb()
	if err != nil {
		fmt.Printf("Error opening db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	cmd := flag.Arg(0)
	fmt.Printf("Command: %s\n", cmd)

	if cmd == "categorizer" {
		err = generateCategorizerDataset(
			db,
			*labelsFile,
			*typesFile,
			*categorizedIdsFile,
			*outFile,
		)
		if err != nil {
			fmt.Printf("Error generating categorizer dataset: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if cmd == "qualitizer" {
		err = generateQualitizerDataset(db, *categorizableIdsFile, *missingInfoIdsFile, *outFile)
		if err != nil {
			fmt.Printf("Error generating qualitizer dataset: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// in red
	fmt.Printf("Unknown command: %s\n", cmd)
	os.Exit(1)

}

func validateFlags() error {
	if *issueDbFile == "" {
		return fmt.Errorf("issueDbFile is required")
	}
	// check that issueDbFile exists
	_, err := os.Stat(*issueDbFile)
	if os.IsNotExist(err) {
		return fmt.Errorf("issueDbFile %s does not exist", *issueDbFile)
	} else if err != nil {
		return err
	}

	if openAiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY env var is required")
	}

	return nil
}

func validateFile(file string) error {
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", file)
	} else if err != nil {
		return err
	}
	return nil
}

func getDb() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", *issueDbFile)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func readFileLines(s string, limit int) ([]string, error) {
	totalLines := 0
	file, err := os.Open(s)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		totalLines++
		if limit > 0 && totalLines >= limit {
			break
		}
	}
	return lines, nil

}

func guaranteeIssuesInDb(db *sql.DB, issueIds []string) {
	var lastRequestTime time.Time = time.Now()
	var requestInterval = time.Second * 1
	didRequest := false
	for _, issueId := range issueIds {
		sleepTime := time.Until(lastRequestTime.Add(requestInterval))
		if sleepTime > 0 && didRequest {
			fmt.Printf("     -> Sleeping for %v\n", sleepTime)
			time.Sleep(sleepTime)
		}
		lastRequestTime = time.Now()
		didRequest = guaranteeIssueInDb(db, issueId)
	}
}

func guaranteeIssueInDb(db *sql.DB, issueId string) bool {
	didRequest := false
	row := db.QueryRow(`SELECT id FROM issues WHERE id = ?`, issueId)
	var id int
	err := row.Scan(&id)
	if err == nil {
		fmt.Println("     -> Issue already exists in db")
		return didRequest
	}

	fmt.Printf("     -> Issue %s not found in db. Fetching from github\n", issueId)

	issueIdInt, err := strconv.Atoi(issueId)
	if err != nil {
		return didRequest
	}
	didRequest = true
	issue, err := github.FetchIssueDetails(issueIdInt)
	if err != nil {
		return didRequest
	}

	labels := []string{}

	for _, label := range issue.Labels {
		labels = append(labels, label.Name)
	}

	labelString := fmt.Sprintf("[%s]", strings.Join(labels, ","))
	issueRaw, err := json.Marshal(issue)
	if err != nil {
		return didRequest
	}

	fmt.Printf("     -> Got issue %s with title: %s\n", issueId, issue.Title)

	_, err = db.Exec(`
		INSERT OR REPLACE INTO issues (id, title, description, processed, labels, raw)
		VALUES (?, ?, ?, ?, ?, ?)
	`, issueId, issue.Title, issue.Body, 0, labelString, string(issueRaw))

	if err != nil {
		return didRequest
	}

	fmt.Println("     -> Inserted issue into db")

	return didRequest
}

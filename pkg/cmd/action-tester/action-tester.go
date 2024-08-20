package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"

	"github.com/grafana/auto-triage/pkg/github"
)

var (
	issueId = flag.Int("issueId", 0, "Github Issue ID (only the number)")
	repo    = flag.String(
		"repo",
		"grafana/grafana-auto-triager-tests",
		"Github repo to push the issue to",
	)
	githubToken = os.Getenv("GH_TOKEN")
	cacheDir    = path.Join(os.TempDir(), "action-tester-cache")
)

func main() {
	flag.Parse()

	err := validateFlags()
	if err != nil {
		fmt.Printf("Error validating flags: %v\n", err)
		os.Exit(1)
	}

	issue, err := getIssueFromCache(*issueId)
	if err != nil {
		fmt.Printf("Reading issue from github\n")
		issue, err = github.FetchGrafanaIssueDetails(*issueId)
		if err != nil {
			fmt.Printf("Error fetching issue details: %v\n", err)
			os.Exit(1)
		}
		err = saveIssueToCache(issue)
		if err != nil {
			fmt.Printf("Error saving issue to cache: %v\n", err)
		}
	}

	newIssue, err := github.PublishIssueToRepo(*repo, issue, []string{"action-tester"})
	if err != nil {
		fmt.Printf("Error publishing issue to repo: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("New issue id: %d\n", newIssue.Number)
	fmt.Printf("New issue url: %s\n", newIssue.HTMLURL)

	fmt.Printf("Issue title: %s\n", issue.Title)
}

func saveIssueToCache(issue github.Issue) error {
	cacheFile := path.Join(cacheDir, strconv.Itoa(issue.Number)+".json")
	fmt.Printf("Saving issue %d to cache file %s\n", issueId, cacheFile)
	issueJson, err := json.Marshal(issue)
	if err != nil {
		fmt.Printf("Error marshalling issue: %v\n", err)
	}
	//create cache dir if it doesn't exist
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		err = os.Mkdir(cacheDir, 0755)
		if err != nil {
			return err
		}
	}

	// save issue to cache
	err = os.WriteFile(cacheFile, issueJson, 0644)
	if err != nil {
		return err
	}

	return nil
}

func getIssueFromCache(issueId int) (github.Issue, error) {
	// check if issue already exists in cache
	cacheFile := path.Join(cacheDir, strconv.Itoa(issueId)+".json")
	fmt.Printf("Checking cache file %s\n", cacheFile)

	if _, err := os.Stat(cacheFile); err == nil {
		fmt.Printf("Issue %d found in cache\n", issueId)
		issue, err := os.ReadFile(cacheFile)
		if err != nil {
			return github.Issue{}, err
		}
		var issueData github.Issue
		err = json.Unmarshal(issue, &issueData)
		if err != nil {
			return github.Issue{}, err
		}
		return issueData, nil
	}

	return github.Issue{}, fmt.Errorf("Issue %d not found in cache", issueId)
}

func validateFlags() error {
	if *issueId == 0 {
		return fmt.Errorf("issueId is required")
	}

	if *repo == "" {
		return fmt.Errorf("repo is required")
	}

	// validate repo format to be org/repo
	re := regexp.MustCompile(`^[a-zA-Z0-9\-]+/[a-zA-Z0-9\-]+$`)
	if !re.MatchString(*repo) {
		return fmt.Errorf("repo format is invalid. Must be org/repo")
	}

	if githubToken == "" {
		return fmt.Errorf("GH_TOKEN env var is required")
	}

	return nil
}

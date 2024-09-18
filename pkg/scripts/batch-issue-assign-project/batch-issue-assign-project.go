package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/grafana/auto-triage/pkg/github"
)

var githubToken = os.Getenv("GH_TOKEN")

var filterTemplate = "label:%s+repo:grafana/grafana+is:issue+is:open+no:project"

var requestInterval = time.Second * 1

// cut date july 30 2024
var cutDate = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

type Command struct {
	Type         string `json:"type"`
	Name         string `json:"name"`
	Action       string `json:"action"`
	AddToProject struct {
		URL string `json:"url"`
	} `json:"addToProject"`
}

func main() {

	genGithubToken, err := github.GetInstallationToken(
		996463,
		"pem-file-path",
		1234,
	)

	if err != nil {
		log.Fatal("Error getting installation token: ", err)
	}

	if githubToken == "" && genGithubToken == "" {
		log.Fatal("GH_TOKEN env var is required")
		return
	}

	githubToken = genGithubToken

	//set GH_TOKEN env var
	os.Setenv("GH_TOKEN", githubToken)

	categories, err := readFileLines("fixtures/categoryLabels.txt")
	if err != nil {
		log.Fatal("Error reading categories: ", err)
	}

	commands, err := getCommands("fixtures/commands.json")
	if err != nil {
		log.Fatal("Error reading commands: ", err)
	}

	log.Printf("Commands: %d\n", len(commands))
	lastRequestTime := time.Now()

	for _, category := range categories {
		filter := fmt.Sprintf(filterTemplate, url.QueryEscape(category))
		page := 1
		log.Printf("Fetching issues for category %s\n", category)
		log.Printf("Filter: %s\n", filter)
		for {

			timeSinceLastRequest := time.Since(lastRequestTime)
			if timeSinceLastRequest < requestInterval {
				log.Printf("  :: Sleeping for %v\n", requestInterval-timeSinceLastRequest)
				time.Sleep(requestInterval - timeSinceLastRequest)
			}
			lastRequestTime = time.Now()

			log.Printf("   :: Fetching page %d\n", page)
			issues, err := github.GetIssuesByFilter(filter, 100, page)
			if err != nil {
				log.Printf("  :: Error getting issues for category %s: %v\n", category, err)
				break
			}

			log.Printf("   ::  Fetched %d issues for category %s\n", len(issues), category)

			if len(issues) == 0 {
				break
			}

			for _, issue := range issues {
				log.Printf(
					"      -> Issue %d: was created at %s. The cut date is %s\n",
					issue.Number,
					issue.CreatedAt,
					cutDate,
				)
				if issue.CreatedAt.Before(cutDate) || issue.AuthorAssociation != "NONE" {
					log.Printf("      x Skipping old issue %d: %s\n", issue.Number, issue.Title)
					continue
				}
				log.Printf("      -> Issue %d: %s\n", issue.Number, issue.Title)
				err = executeCommandsForIssue(issue, commands)
				if err != nil {
					log.Fatalf("Error executing commands for issue %d: %v\n", issue.Number, err)
				}
				os.Exit(0)
			}

			// do not request more if there are less than 100 issues (last page)
			if len(issues) < 100 {
				break
			}

			page++
		}
	}
}

func executeCommandsForIssue(issue github.Issue, commands []Command) error {

	//pretty print all issue object
	// fmt.Printf("%+v\n", issue)

	for _, label := range issue.Labels {
		for _, command := range commands {
			if command.Name == label.Name {
				log.Printf(
					"                  :-> Adding project %s to issue %d\n",
					command.AddToProject.URL,
					issue.Number,
				)

				err := assignProjectToIssue(issue, command.AddToProject.URL)
				if err != nil {
					return fmt.Errorf("error assigning project to issue: %v", err)
				}
			}
		}
	}
	return nil
}

var projectNodeIds = make(map[string]string)

func assignProjectToIssue(issue github.Issue, projectUrl string) error {
	// project url is like https://github.com/orgs/grafana/projects/69
	re := regexp.MustCompile(`https://github.com/orgs/.*?/projects/(\d+)`)
	matches := re.FindStringSubmatch(projectUrl)
	if len(matches) != 2 {
		return fmt.Errorf("invalid project url: %s", projectUrl)
	}

	projectId, err := strconv.Atoi(matches[1])
	if err != nil {
		return err
	}

	projectNodeId, ok := projectNodeIds[projectUrl]
	if !ok {
		projectNodeId, err = github.GetProjectNodeId("grafana", projectId)
		if err != nil {
			return err
		}

		log.Printf(
			"                  :-> Project node id for project %s is %s\n",
			projectUrl,
			projectNodeId,
		)
		projectNodeIds[projectUrl] = projectNodeId
	}

	log.Printf(
		"                  :-> Assigning project %s with node id %s to issue %d\n",
		projectUrl,
		projectNodeId,
		issue.Number,
	)

	// confirm with user
	fmt.Printf("Do you want to assign project %s to issue %d? (y/n): ", projectUrl, issue.Number)
	var response string
	_, err = fmt.Scanln(&response)
	if err != nil {
		return err
	}
	if response != "y" {
		return fmt.Errorf("user declined to assign project")
	}

	err = github.AssignProjectToIssue(issue.NodeID, projectNodeId)
	if err != nil {
		return err
	}

	return nil
}

func readFileLines(s string) ([]string, error) {
	file, err := os.Open(s)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, nil

}

func getCommands(file string) ([]Command, error) {
	var commands []Command

	content, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(content, &commands)
	if err != nil {
		return nil, err
	}

	return commands, nil
}

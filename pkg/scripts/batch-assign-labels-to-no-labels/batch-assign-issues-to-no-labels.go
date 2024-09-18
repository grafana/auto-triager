package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/grafana/auto-triage/pkg/github"
)

var githubToken = os.Getenv("GH_TOKEN")
var openAiKey = os.Getenv("OPENAI_API_KEY")
var rateLimit = time.Second * 10

func main() {

	genGithubToken, err := github.GetInstallationToken(
		996463,
		"pem-file",
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

	filter := "repo:grafana/grafana+is:open+is:issue+no:label"

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("Error getting current working directory: ", err)
	}

	triagerBinary := path.Join(cwd, "bin", "linux_amd64", "triager-openai")

	stat, err := os.Stat(triagerBinary)
	if err != nil {
		log.Fatal("Error getting triager binary: ", err)
	}

	// add execute permission
	err = os.Chmod(triagerBinary, stat.Mode()|os.ModeSetuid)
	if err != nil {
		log.Fatal("Error setting execute permission on triager binary: ", err)
	}

	env := map[string]string{
		"GH_TOKEN":       githubToken,
		"OPENAI_API_KEY": openAiKey,
		"DEBUG":          "1",
	}

	for {
		issues, err := github.GetIssuesByFilter(filter, 100, 1)
		if err != nil {
			log.Fatal("Error getting issues: ", err)
		}

		if len(issues) == 0 {
			break
		}

		lastRequestTime := time.Now()

		for _, issue := range issues {
			// if issue.AuthorAssociation != "NONE" {
			// 	fmt.Printf(
			// 		"\nSkipping issue %d: %s because it's author_association is %s\n",
			// 		issue.Number,
			// 		issue.Title,
			// 		issue.AuthorAssociation,
			// 	)
			// 	continue
			// }

			timeSinceLastRequest := time.Since(lastRequestTime)
			if timeSinceLastRequest < rateLimit {
				fmt.Printf("\n     -> Sleeping for %v\n", rateLimit-timeSinceLastRequest)
				time.Sleep(rateLimit - timeSinceLastRequest)
			}
			lastRequestTime = time.Now()

			// confirm := "n"
			fmt.Printf("\n-----\nIssue %d: %s. %s\n", issue.Number, issue.Title, issue.HTMLURL)
			// fmt.Printf("Do you want to run triager on this issue? (y/n): ")
			// _, err = fmt.Scanln(&confirm)
			// if err != nil {
			// 	log.Fatal("Error reading input: ", err)
			// }
			// if confirm != "y" {
			// 	continue
			// }

			fmt.Printf("Issue %d: %s\n", issue.Number, issue.Title)
			// run triager
			cmd := exec.Command(
				triagerBinary,
				"-issueId",
				fmt.Sprintf("%d", issue.Number),
				"-addLabels=true",
			)
			cmd.Env = os.Environ()
			for key, value := range env {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				log.Fatal("Error running triager: ", err)
			}
		}

		break

	}
}

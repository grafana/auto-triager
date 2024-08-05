package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/grafana/auto-triage/pkg/github"
	"github.com/sashabaranov/go-openai"
)

var (
	openAiKey = os.Getenv("OPENAI_API_KEY")
	ghToken   = os.Getenv("GH_TOKEN")
	issueId   = flag.Int("issueId", 0, "Github Issue ID (only the number)")
	model     = flag.String(
		"model",
		// "ft:gpt-4o-mini-2024-07-18:grafana-labs-experiments-exploration:auto-triage:9ssSMoCP", // 800k training tokens
		"ft:gpt-4o-mini-2024-07-18:grafana-labs-experiments-exploration:auto-triage:9ss5MNR0", // 400k training tokens
		"Model to use",
	)
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
)

func main() {
	flag.Parse()

	err := validateFlags()
	if err != nil {
		fmt.Printf("Error validating flags: %v\n", err)
		os.Exit(1)
	}

	issueData, err := github.FetchIssueDetails(*issueId)
	if err != nil {
		log.Fatal("Error fetching issue details: ", err)
	}

	if issueData.Title == "" {
		log.Fatal("Error fetching issue details: Title is empty")
		os.Exit(1)
	}

	fmt.Printf(":: Got issue %d with title: %s\n", *issueId, issueData.Title)

	areaLabels, err := readFileLines(*labelsFile)
	if err != nil {
		log.Fatal("Error reading areaLabels.txt: ", err)
	}

	typeLabels, err := readFileLines(*typesFile)
	if err != nil {
		log.Fatal("Error reading typeLabels.txt: ", err)
	}

	var systemPrompt = `
			You are an expert Grafana issues categorizer. 
			You are provided with a Grafana issue. 
			You will categorize the issue into one of the provided list of types and areas. 

			It is possible that there are multiple areas and types for a given issue or none at all. 
			In that case you should return an empty array for the specific field.

			The output should be a valid json object with the following fields: 
			* id: The id of the current issue 
			* areaLabel: The area label of the current issue 
			* typeLabel: The type of the current issue 

			### Start of list of types
			` + strings.Join(typeLabels, "\n") +
		`
			### End of list of types

			
			### Start of list of areas
			This is the list of areas:
			` + strings.Join(areaLabels, "\n") +
		`
			### End of list of areas
			`

	client := openai.NewClient(openAiKey)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: *model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role: openai.ChatMessageRoleUser,
					Content: `
					  Issue ID: ` + strconv.Itoa(*issueId) + `
					  Issue title: ` + issueData.Title + `
					  Issue description:\n\n ` + issueData.Body + `
					`,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(resp.Choices[0].Message.Content)

}

func validateFlags() error {
	if *issueId == 0 {
		return fmt.Errorf("issueId is required")
	}

	if openAiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY env var is required")
	}

	if ghToken == "" {
		return fmt.Errorf("GH_TOKEN env var is required")
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

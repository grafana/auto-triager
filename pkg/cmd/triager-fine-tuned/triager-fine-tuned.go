package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/grafana/auto-triage/pkg/github"
	"github.com/grafana/auto-triage/pkg/prompts"
	"github.com/sashabaranov/go-openai"
)

type QualityVeredict struct {
	IsCategorizable bool        `json:"isCategorizable"`
	ID              interface{} `json:"id"`
	Remarks         string      `json:"remarks"`
}

type CategorizedIssue struct {
	ID              interface{} `json:"id"`
	AreaLabel       []string    `json:"areaLabel"`
	TypeLabel       []string    `json:"typeLabel"`
	IsCategorizable bool        `json:"isCategorizable"`
	Remarks         string      `json:"remarks"`
}

var (
	openAiKey        = os.Getenv("OPENAI_API_KEY")
	ghToken          = os.Getenv("GH_TOKEN")
	issueId          = flag.Int("issueId", 0, "Github Issue ID (only the number)")
	categorizerModel = flag.String(
		"categorizerModel",
		// "ft:gpt-4o-mini-2024-07-18:grafana-labs-experiments-exploration:auto-triage:9ssSMoCP", // 800k training tokens
		"ft:gpt-4o-mini-2024-07-18:grafana-labs-experiments-exploration:auto-triage:9ss5MNR0", // 400k training tokens
		"Model to use",
	)
	qualitizerModel = flag.String(
		"qualitizerModel",
		// "ft:gpt-4o-mini-2024-07-18:grafana-labs-experiments-exploration:issue-qualitizer:9tDkW7Kq", // first training
		"gpt-4o",
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

	areaLabels, err := readFileLines(*labelsFile)
	if err != nil {
		log.Fatal("Error reading areaLabels.txt: ", err)
	}

	typeLabels, err := readFileLines(*typesFile)
	if err != nil {
		log.Fatal("Error reading typeLabels.txt: ", err)
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

	fmt.Printf(":: Checking if issue can be categorized\n")

	qualityVeredict, err := getIssueIsCategorizable(&issueData, qualitizerModel)

	if err != nil {
		log.Fatal("Error judging issue quality: ", err)
		os.Exit(1)
	}

	fmt.Printf("Is categorizable: %s\n", strconv.FormatBool(qualityVeredict.IsCategorizable))

	fmt.Printf(":: Categorizing issue\n")

	category, err := getIssueCategory(&issueData, categorizerModel, typeLabels, areaLabels)
	if err != nil {
		log.Fatal("Error categorizing issue: ", err)
		os.Exit(1)
	}

	fmt.Printf("Categorizaion: %v\n", category)

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

func getIssueCategory(
	issueData *github.Issue,
	model *string,
	typeLabels []string,
	areaLabels []string,
) (CategorizedIssue, error) {
	var categoryzerPrompt = prompts.CategorySystemPrompt + `

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
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: categoryzerPrompt,
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
		return CategorizedIssue{}, err
	}

	category := CategorizedIssue{}
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &category)
	if err != nil {
		return CategorizedIssue{}, err
	}

	return category, nil

}

func getIssueIsCategorizable(issueData *github.Issue, model *string) (QualityVeredict, error) {
	client := openai.NewClient(openAiKey)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: *model,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: prompts.QualitySystemPrompt,
				},
				{
					Role: openai.ChatMessageRoleUser,
					Content: `
					  Issue ID: ` + strconv.Itoa(issueData.Number) + `
					  Issue title: ` + issueData.Title + `
					  Issue description:\n\n ` + issueData.Body + `
					`,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return QualityVeredict{}, err
	}

	fmt.Println(resp.Choices[0].Message.Content)

	veredict := QualityVeredict{}
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &veredict)
	if err != nil {
		return QualityVeredict{}, err
	}

	return veredict, nil

}

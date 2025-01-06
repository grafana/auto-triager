package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/auto-triage/pkg/github"
	"github.com/grafana/auto-triage/pkg/logme"
	"github.com/grafana/auto-triage/pkg/prompts"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	"github.com/tiktoken-go/tokenizer"
)

type QualityVeredict struct {
	IsCategorizable bool        `json:"isCategorizable"`
	ID              interface{} `json:"id"`
	Remarks         string      `json:"remarks"`
}

type CategorizedIssue struct {
	ID              interface{} `json:"id"`
	CategoryLabel   []string    `json:"categoryLabel"`
	TypeLabel       []string    `json:"typeLabel"`
	IsCategorizable bool        `json:"isCategorizable"`
	Remarks         string      `json:"remarks"`
}

var (
	openAiKey        = os.Getenv("OPENAI_API_KEY")
	ghToken          = os.Getenv("GH_TOKEN")
	issueId          = flag.Int("issueId", 0, "Github Issue ID (only the number)")
	repo             = flag.String("repo", "grafana/grafana", "Github repo to push the issue to")
	categorizerModel = flag.String(
		"categorizerModel",
		"gpt-4o", // regular model from openai
		"Model to use",
	)
	addLabels = flag.Bool(
		"addLabels",
		false,
		"Add labels to the issue in the repo via the GitHub API",
	)
	retries = flag.Int(
		"retries",
		5,
		"Number of retries to use when categorizing an issue",
	)
	labelsFile = flag.String(
		"labelsFile",
		"fixtures/categoryLabels.txt",
		"Labels file. One label per line",
	)
	typesFile = flag.String(
		"typesFile",
		"fixtures/typeLabels.txt",
		"Types file. One label per line",
	)
)

func main() {
	var err error

	flag.Parse()

	err = validateFlags()
	if err != nil {
		logme.FatalF("Error validating flags: %v\n", err)
	}

	categoryLabels, err := readFileLines(*labelsFile)
	if err != nil {
		logme.FatalF("Error reading categoryLabels.txt: %v\n", err)
	}

	typeLabels, err := readFileLines(*typesFile)
	if err != nil {
		logme.FatalF("Error reading typeLabels.txt: %v\n", err)
	}

	issueData, err := github.FetchIssueDetails(*issueId, *repo)
	if err != nil {
		logme.FatalF("Error fetching issue details: %v\n", err)
	}

	if issueData.Title == "" {
		logme.FatalLn("Error fetching issue details: Title is empty")
	}

	// logme.InfoF(":: Checking if issue can be categorized\n")

	// qualityVeredict, err := getIssueIsCategorizable(&issueData, qualitizerModel)
	//
	// if err != nil {
	// 	log.Fatal("Error judging issue quality: ", err)
	// 	os.Exit(1)
	// }
	//
	// logme.InfoF("Is categorizable: %s\n", strconv.FormatBool(qualityVeredict.IsCategorizable))

	logme.InfoF(":: Categorizing issue\n")
	logme.DebugF("Repo: %s\n", *repo)
	logme.DebugF("Issue ID: %d\n", *issueId)
	logme.DebugF("Model: %s\n", *categorizerModel)
	logme.DebugF("Issue title: %s\n", issueData.Title)

	leftRetries := *retries
	category := CategorizedIssue{}

	for leftRetries > 0 {
		category, err = getIssueCategory(&issueData, categorizerModel, typeLabels, categoryLabels)
		if err != nil || category.ID == 0 || category.ID == nil {
			retriesLeft := leftRetries - 1
			logme.ErrorF("Error categorizing issue: %v\n", err)
			logme.InfoF("Retrying in 1 second. %d retries left\n", retriesLeft)
			time.Sleep(time.Second)
			leftRetries = retriesLeft
			continue
		}

		// filter out the categories that are not in the categoryLabels
		realCategories := []string{}
		for _, category := range category.CategoryLabel {
			if slices.Contains(categoryLabels, category) {
				realCategories = append(realCategories, category)
			} else {
				logme.DebugF("Category %s is not in categoryLabels. Skipping", category)
			}
		}

		if len(realCategories) == 0 {
			logme.ErrorF("Error categorizing issue: Model returned only false categories")
			retriesLeft := leftRetries - 1
			logme.InfoF("Retrying in 1 second. %d retries left\n", retriesLeft)
			time.Sleep(time.Second)
			leftRetries = retriesLeft
			continue
		}

		category.CategoryLabel = realCategories

		break
	}

	if leftRetries == 0 && err != nil {
		logme.FatalF("Error categorizing issue: %v\n", err)
	}

	logme.InfoF("Finished categorizing issue")

	if *addLabels {
		logme.InfoF("Adding labels to issue")

		labels := []string{}
		labels = append(labels, category.CategoryLabel...)
		labels = append(labels, category.TypeLabel...)
		labels = append(labels, "automated-triage")
		err = github.AddLabelsToIssue(*repo, *issueId, labels)
		if err != nil {
			logme.FatalF("Error adding labels to issue: %v\n", err)
		}
		logme.InfoF("Finished adding labels to issue")
	}

	categoryJson, err := json.Marshal(category)
	if err != nil {
		logme.FatalF("Error marshalling category: %v\n", err)
	}

	fmt.Printf("%s", categoryJson)

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
	categoryLabels []string,
) (CategorizedIssue, error) {
	var categoryzerPrompt = prompts.CategorySystemPrompt

	// calculate the number of tokens
	enc, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		return CategorizedIssue{}, err
	}

	tokens, _, err := enc.Encode(categoryzerPrompt)
	if err != nil {
		return CategorizedIssue{}, err
	}

	logme.DebugF("Tokens: %d\n", len(tokens))

	// set up structured output schema
	type Result struct {
		ID              int      `json:"id"`
		IsCategorizable bool     `json:"isCategorizable"`
		Remarks         string   `json:"remarks"`
		CategoryLabel   []string `json:"categoryLabel"`
		TypeLabel       []string `json:"typeLabel"`
	}

	var result Result
	schema, err := jsonschema.GenerateSchemaForType(result)
	if err != nil {
		log.Fatalf("GenerateSchemaForType error: %v", err)
	}

	client := openai.NewClient(openAiKey)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: *model,
			// ResponseFormat: &openai.ChatCompletionResponseFormat{
			// 	Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			// },
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

						According to the following list, which category and type do you think this issue belongs to?

					List of categories:
					` + strings.Join(categoryLabels, "\n") +
						`
					List of types: ` + strings.Join(typeLabels, "\n"),
				},
			},
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
				JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
					Name:   "math_reasoning",
					Schema: schema,
					Strict: true,
				},
			},
		},
	)

	if err != nil {
		logme.FatalF("ChatCompletion error: %v\n", err)
		return CategorizedIssue{}, err
	}

	category := CategorizedIssue{}
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &category)
	if err != nil {
		return CategorizedIssue{}, fmt.Errorf("error unmarshaling issue category: %w", err)
	}

	return category, nil

}

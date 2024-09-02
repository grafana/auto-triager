package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/auto-triage/pkg/github"
	"github.com/grafana/auto-triage/pkg/logme"
	"github.com/grafana/auto-triage/pkg/prompts"
	"github.com/sashabaranov/go-openai"
	"github.com/tiktoken-go/tokenizer"
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
	repo             = flag.String("repo", "grafana/grafana", "Github repo to push the issue to")
	categorizerModel = flag.String(
		"categorizerModel",
		// "ft:gpt-4o-mini-2024-07-18:grafana-labs-experiments-exploration:auto-triage:9ssSMoCP", // 800k training tokens
		"ft:gpt-4o-mini-2024-07-18:grafana-labs-experiments-exploration:auto-triage:9ss5MNR0", // 400k training tokens
		// "ft:gpt-4o-2024-08-06:grafana-labs-experiments-exploration:issue-auto-triager:9yxdY5IU", // 400k training tokens gpt-4o
		"Model to use",
	)
	// qualitizerModel = flag.String(
	// 	"qualitizerModel",
	// 	// "ft:gpt-4o-mini-2024-07-18:grafana-labs-experiments-exploration:issue-qualitizer:9tDkW7Kq", // first training. Bad model
	// 	// "gpt-4o", // regular model from openai, produces remarks but it not fine tuned
	// 	// "ft:gpt-4o-mini-2024-07-18:grafana-labs-experiments-exploration:issue-qualitizer:9tW2EBhH", // good. trained with a larger dataset but doesn't produce remarks
	// 	"ft:gpt-4o-mini-2024-07-18:grafana-labs-experiments-exploration:issue-qualitizer:9tWTKBHh", // good. trained with a larger dataset. might produce remarks but most of the time it doesn't
	// 	"Model to use",
	// )
	addLabels = flag.Bool(
		"addLabels",
		false,
		"Add labels to the issue in the repo via the GitHub API",
	)
	retries = flag.Int(
		"retries",
		3,
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

	areaLabels, err := readFileLines(*labelsFile)
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
		category, err = getIssueCategory(&issueData, categorizerModel, typeLabels, areaLabels)
		if err != nil || category.ID == 0 || category.ID == nil {
			retriesLeft := leftRetries - 1
			logme.ErrorF("Error categorizing issue: %v\n", err)
			logme.InfoF("Retrying in 1 second. %d retries left\n", retriesLeft)
			time.Sleep(time.Second)
			leftRetries = retriesLeft
			continue
		}

		break
	}

	if leftRetries == 0 && err != nil {
		logme.FatalF("Error categorizing issue: %v\n", err)
	}

	logme.InfoF("Finished categorizing issue")

	if *addLabels {
		logme.InfoF("Adding labels to issue")

		labels := []string{}
		labels = append(labels, category.AreaLabel...)
		labels = append(labels, category.TypeLabel...)
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

// func getIssueIsCategorizable(issueData *github.Issue, model *string) (QualityVeredict, error) {
// 	client := openai.NewClient(openAiKey)
// 	resp, err := client.CreateChatCompletion(
// 		context.Background(),
// 		openai.ChatCompletionRequest{
// 			Model: *model,
// 			ResponseFormat: &openai.ChatCompletionResponseFormat{
// 				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
// 			},
// 			Messages: []openai.ChatCompletionMessage{
// 				{
// 					Role:    openai.ChatMessageRoleSystem,
// 					Content: prompts.QualitySystemPrompt,
// 				},
// 				{
// 					Role: openai.ChatMessageRoleUser,
// 					Content: `
// 					  Issue ID: ` + strconv.Itoa(issueData.Number) + `
// 					  Issue title: ` + issueData.Title + `
// 					  Issue description:\n\n ` + issueData.Body + `
// 					`,
// 				},
// 			},
// 		},
// 	)
// 	if err != nil {
// 		Debug.Printf("ChatCompletion error: %v\n", err)
// 		return QualityVeredict{}, err
// 	}
//
// 	Debug.Println(resp.Choices[0].Message.Content)
//
// 	veredict := QualityVeredict{}
// 	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &veredict)
// 	if err != nil {
// 		return QualityVeredict{}, err
// 	}
//
// 	return veredict, nil
//
// }

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/grafana/auto-triage/pkg/github"
	"github.com/grafana/auto-triage/pkg/prompts"
	"github.com/sashabaranov/go-openai"
	"github.com/tiktoken-go/tokenizer"
)

var Debug *log.Logger

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

	if os.Getenv("DEBUG") == "1" {
		Debug = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
	} else {
		Debug = log.New(io.Discard, "", 0)
	}

	err := validateFlags()
	if err != nil {
		Debug.Fatalf("Error validating flags: %v\n", err)
		os.Exit(1)
	}

	areaLabels, err := readFileLines(*labelsFile)
	if err != nil {
		Debug.Fatalf("Error reading areaLabels.txt: %v\n", err)
	}

	typeLabels, err := readFileLines(*typesFile)
	if err != nil {
		Debug.Fatalf("Error reading typeLabels.txt: %v\n", err)
	}

	issueData, err := github.FetchIssueDetails(*issueId, *repo)
	if err != nil {
		Debug.Fatalf("Error fetching issue details: %v\n", err)
	}

	if issueData.Title == "" {
		Debug.Fatal("Error fetching issue details: Title is empty")
		os.Exit(1)
	}

	Debug.Printf(":: Got issue %d with title: %s\n", *issueId, issueData.Title)
	// Debug.Printf(":: Checking if issue can be categorized\n")

	// qualityVeredict, err := getIssueIsCategorizable(&issueData, qualitizerModel)
	//
	// if err != nil {
	// 	log.Fatal("Error judging issue quality: ", err)
	// 	os.Exit(1)
	// }
	//
	// Debug.Printf("Is categorizable: %s\n", strconv.FormatBool(qualityVeredict.IsCategorizable))

	Debug.Printf(":: Categorizing issue\n")

	category, err := getIssueCategory(&issueData, categorizerModel, typeLabels, areaLabels)
	if err != nil {
		Debug.Fatalf("Error categorizing issue: %v\n", err)
	}

	fmt.Printf("%v", category)

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

	Debug.Printf("Tokens: %d\n", len(tokens))

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
		Debug.Printf("ChatCompletion error: %v\n", err)
		return CategorizedIssue{}, err
	}

	category := CategorizedIssue{}
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &category)
	if err != nil {
		return CategorizedIssue{}, err
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

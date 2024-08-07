package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/grafana/auto-triage/pkg/prompts"
	"github.com/tiktoken-go/tokenizer"
)

var qualitizerSystemPrompt = PromptMessage{
	Role:    "system",
	Content: prompts.QualitySystemPrompt,
}

func generateQualitizerDataset(
	db *sql.DB,
	categorizableIdsFile string,
	missingInfoIdsFile string,
	outFile string,
) error {

	categorizableIds, err := readFileLines(categorizableIdsFile, 100)
	if err != nil {
		return err
	}

	missingInfoIds, err := readFileLines(missingInfoIdsFile, 100)
	if err != nil {
		return err
	}

	//join them
	guaranteeIssuesInDb(db, categorizableIds)
	guaranteeIssuesInDb(db, missingInfoIds)

	var finalPrompts []PromptTemplate

	fmt.Printf(
		"Generating qualitizer dataset for categorizable issues with %d ids\n",
		len(categorizableIds),
	)
	prompts, err := generatePrompts(db, categorizableIds, true, maxTokens/2)
	if err != nil {
		return err
	}
	finalPrompts = append(finalPrompts, prompts...)

	fmt.Printf(
		"Generating qualitizer dataset for missing info issues with %d ids\n",
		len(missingInfoIds),
	)
	prompts, err = generatePrompts(db, missingInfoIds, false, maxTokens/2)
	if err != nil {
		return err
	}
	finalPrompts = append(finalPrompts, prompts...)

	var finalContent string = ""
	for _, prompt := range finalPrompts {
		content, err := json.Marshal(prompt)
		if err != nil {
			return err
		}
		finalContent += string(content) + "\n"
	}

	err = os.WriteFile(outFile, []byte(finalContent), 0644)
	if err != nil {
		return err
	}

	return nil

}

func generatePrompts(
	db *sql.DB,
	allIds []string,
	isCategorizable bool,
	maxTokens int,
) ([]PromptTemplate, error) {
	var finalPrompts []PromptTemplate
	var totalTokens int
	var totalIssues = 0

	sql := `
        SELECT id, title, description FROM issues WHERE processed = 0 AND id IN (` + strings.Join(allIds, ",") + `)
    `

	enc, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var prompt PromptTemplate
		var id int
		var title string
		var description string

		prompt.Messages = append(prompt.Messages, qualitizerSystemPrompt)
		err = rows.Scan(&id, &title, &description)
		if err != nil {
			return nil, err
		}

		fmt.Printf("Processing issue %d\n", id)

		prompt.Messages = append(prompt.Messages, PromptMessage{Role: "user", Content: `
            Issue ID: ` + strconv.Itoa(id) + `
            Issue title: ` + title + `
            Issue description:\n\n ` + description + `
        `})

		jsonResponse := `{
            "id": ` + strconv.Itoa(id) + `,
            "isCategorizable":  ` + strconv.FormatBool(isCategorizable) + `
        }`

		jsonResponse = strings.ReplaceAll(jsonResponse, "\n", "")
		jsonResponse = strings.ReplaceAll(jsonResponse, "\t", "")
		jsonResponse = strings.ReplaceAll(jsonResponse, " ", "")

		prompt.Messages = append(
			prompt.Messages,
			PromptMessage{
				Role:    "assistant",
				Content: jsonResponse,
			},
		)

		promptJson, err := json.Marshal(prompt)
		if err != nil {
			fmt.Printf("Error marshalling prompt: %v\n", err)
			continue
		}

		tokens, _, err := enc.Encode(string(promptJson))
		if err != nil {
			fmt.Printf("Error encoding prompt: %v\n", err)
			continue
		}

		if (totalTokens + len(tokens)) > maxTokens {
			fmt.Printf("Reached max tokens. Stopping\n")
			break
		}
		totalIssues++
		totalTokens += len(tokens)
		fmt.Printf("Total tokens so far: %d\n", totalTokens)

		finalPrompts = append(finalPrompts, prompt)
	}

	fmt.Printf("Total tokens so far: %d\n", totalTokens)
	fmt.Printf("Total issues categorizable: %d\n", totalIssues)

	return finalPrompts, nil
}

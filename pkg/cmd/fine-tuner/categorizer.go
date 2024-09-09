package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/grafana/auto-triage/pkg/prompts"
	"github.com/tiktoken-go/tokenizer"
)

type JsonPrompt struct {
	Question string `json:"question"`
	Context  string `json:"context"`
	Answer   string `json:"answer"`
	System   string `json:"system"`
}

func generateCategorizerDataset(
	db *sql.DB,
	labelsFile string,
	typesFile string,
	idsFile string,
	outFile string,
) error {

	enc, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		return err
	}

	categories, err := readFileLines(labelsFile, 0)
	if err != nil {
		return err
	}

	types, err := readFileLines(typesFile, 0)
	if err != nil {
		return err
	}

	err = validateFile(idsFile)
	if err != nil {
		return err
	}

	var categorizerSystemPrompt = PromptMessage{
		Role:    "system",
		Content: prompts.CategorySystemPrompt,
	}

	ids, err := readFileLines(idsFile, 0)
	if err != nil {
		return err
	}

	fmt.Printf("Generating dataset with %d ids\n", len(ids))

	sql := `
		SELECT id, title, description, labels FROM issues WHERE id IN (` + strings.Join(ids, ",") + `)
	`
	fmt.Printf("SQL: %s\n", sql)

	rows, err := db.Query(sql)
	if err != nil {
		return err
	}
	defer rows.Close()

	var finalPrompts []PromptTemplate
	var totalTokens int
	var totalIssues = 0

	jsonPrompts := []JsonPrompt{}

	for rows.Next() {
		var prompt PromptTemplate
		prompt.Messages = append(prompt.Messages, categorizerSystemPrompt)
		var id int
		var title string
		var description string
		var labels string
		err = rows.Scan(&id, &title, &description, &labels)
		if err != nil {
			return err
		}

		fmt.Printf("Processing issue %d\n", id)
		prompt.Messages = append(prompt.Messages, PromptMessage{Role: "user", Content: `
			Issue ID: ` + strconv.Itoa(id) + `
			Issue title: ` + title + `
			Issue description:\n\n ` + description + `

			According to the following list, which category and type do you think this issue belongs to?

					List of categories:
					` + strings.Join(categories, "\n") +
			`
					List of types: ` + strings.Join(types, "\n"),
		})

		preCategoryLabels, preTypeLabels, err := getLabelsFromIssueLabels(labels)
		if err != nil {
			continue
		}

		// filter out the categories that are not in the categoryLabels
		categoryLabels := []string{}
		for _, category := range preCategoryLabels {
			if slices.Contains(categories, category) {
				categoryLabels = append(categoryLabels, category)
			}
		}

		// filter out the types that are not in the typeLabels
		typeLabels := []string{}
		for _, typeLabel := range preTypeLabels {
			if slices.Contains(types, typeLabel) {
				typeLabels = append(typeLabels, typeLabel)
			}
		}

		// do not use examples without labels
		if len(categoryLabels) == 0 || len(typeLabels) == 0 ||
			len(categoryLabels) != len(typeLabels) {
			continue
		}

		jsonPrompt := JsonPrompt{
			Question: `
						Issue ID: ` + strconv.Itoa(id) + `
						Issue title: ` + title + `
						Issue description:\n\n ` + description + `

						According to the following list, which category and type do you think this issue belongs to?
			`,
			Context: `
						List of categories:
						` + strings.Join(categories, "\n") +
				`
						List of types: ` + strings.Join(types, "\n"),
			Answer: `{
				"id": ` + strconv.Itoa(id) + `,
				"categoryLabel":` + stringArrayToJsonArray(categoryLabels) + `,
				"typeLabel": ` + stringArrayToJsonArray(typeLabels) + `
			}`,
			System: prompts.CategorySystemPrompt,
		}

		jsonPrompts = append(jsonPrompts, jsonPrompt)

		jsonResponse := `{
			"id": ` + strconv.Itoa(id) + `,
			"categoryLabel":` + stringArrayToJsonArray(categoryLabels) + `,
			"typeLabel": ` + stringArrayToJsonArray(typeLabels) + `
		}`

		jsonResponse = strings.ReplaceAll(jsonResponse, "\n", "")
		jsonResponse = strings.ReplaceAll(jsonResponse, "\t", "")
		jsonResponse = strings.ReplaceAll(jsonResponse, " ", "")

		prompt.Messages = append(
			prompt.Messages,
			PromptMessage{
				Role: "assistant",
				// without line breaks or spaces
				Content: jsonResponse,
			},
		)

		promptJson, err := json.Marshal(prompt)
		if err != nil {
			continue
		}

		tokens, _, err := enc.Encode(string(promptJson))
		if err != nil {
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

	fmt.Printf("Total tokens: %d\n", totalTokens)
	fmt.Printf("Total issues: %d\n", totalIssues)

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

	jsonFinalContent, err := json.Marshal(jsonPrompts)
	if err != nil {
		return err
	}

	err = os.WriteFile(outFile+".json", []byte(jsonFinalContent), 0644)
	if err != nil {
		return err
	}

	return nil
}

func getLabelsFromIssueLabels(labels string) ([]string, []string, error) {
	var categoryLabels []string
	var typeLabels []string

	//labels if not empty is a json array
	if labels != "" {
		var parsedLabels []string
		err := json.Unmarshal([]byte(labels), &parsedLabels)
		if err != nil {
			return nil, nil, err
		}

		for _, label := range parsedLabels {
			if strings.HasPrefix(label, "area/") || strings.HasPrefix(label, "datasource/") {
				categoryLabels = append(categoryLabels, label)
			} else if strings.HasPrefix(label, "type/") {
				typeLabels = append(typeLabels, label)
			}
		}
	}

	return categoryLabels, typeLabels, nil

}

func stringArrayToJsonArray(array []string) string {
	if len(array) == 0 {
		return ""
	}
	var jsonArray []string
	for _, label := range array {
		jsonArray = append(jsonArray, fmt.Sprintf(`"%s"`, label))
	}
	return fmt.Sprintf("[%s]", strings.Join(jsonArray, ","))
}

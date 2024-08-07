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

	labels, err := readFileLines(labelsFile, 0)
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
		Role: "system",
		Content: prompts.CategorySystemPrompt + `

			### Start of list of types
			` + strings.Join(types, "\n") +
			`
			### End of list of types

			
			### Start of list of areas
			This is the list of areas:
			` + strings.Join(labels, "\n") +
			`
			### End of list of areas
			`,
	}

	ids, err := readFileLines(idsFile, 0)
	if err != nil {
		return err
	}

	fmt.Printf("Generating dataset with %d ids\n", len(ids))

	sql := `
		SELECT id, title, description, labels FROM issues WHERE processed = 0 AND id IN (` + strings.Join(ids, ",") + `)
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
		prompt.Messages = append(prompt.Messages, PromptMessage{Role: "user", Content: `
			Issue ID: ` + strconv.Itoa(id) + `
			Issue title: ` + title + `
			Issue description:\n\n ` + description + `
		`})

		areaLabels, typeLabels, err := getLabelsFromIssueLabels(labels)
		if err != nil {
			continue
		}

		// do not use examples without labels
		if len(areaLabels) == 0 || len(typeLabels) == 0 {
			continue
		}

		jsonResponse := `{
			"id": ` + strconv.Itoa(id) + `,
			"areaLabel": [` + strings.Join(areaLabels, ",") + `],
			"typeLabel": [` + strings.Join(typeLabels, ",") + `]
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

	return nil
}

func getLabelsFromIssueLabels(labels string) ([]string, []string, error) {
	var areaLabels []string
	var typeLabels []string

	//labels if not empty is a json array
	if labels != "" {
		var parsedLabels []string
		err := json.Unmarshal([]byte(labels), &parsedLabels)
		if err != nil {
			return nil, nil, err
		}

		for _, label := range parsedLabels {
			if strings.HasPrefix(label, "area/") {
				areaLabels = append(areaLabels, fmt.Sprintf(`"%s"`, label))
			} else if strings.HasPrefix(label, "type/") {
				typeLabels = append(typeLabels, fmt.Sprintf(`"%s"`, label))
			}
		}
	}

	return areaLabels, typeLabels, nil

}

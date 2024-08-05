package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tiktoken-go/tokenizer"

	"flag"
	"fmt"
	"os"
)

type PromptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type PromptTemplate struct {
	Messages []PromptMessage `json:"messages"`
}

var (
	issueDbFile = flag.String("issuesDb", "github-data.sqlite", "Issue database file")
	idsFile     = flag.String(
		"idsFile",
		"",
		"File containing the ids of the issues to process. One per line",
	)
	openAiKey  = os.Getenv("OPENAI_API_KEY")
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
	outFile = flag.String("outFile", "fine-tune-dataset.json", "Output file")
)

var availableCommands = []string{"gen-dataset"}
var maxTokens = 100000

func main() {
	var err error
	flag.Parse()

	err = validateFlags()
	if err != nil {
		fmt.Printf("Error validating flags: %v\n", err)
		os.Exit(1)
	}

	db, err := getDb()
	if err != nil {
		fmt.Printf("Error opening db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	command, err := getCommand()
	if err != nil {
		fmt.Printf("Error getting command: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Command: %s\n", command)

	if command == "gen-dataset" {
		err = generateDataset(db)
		if err != nil {
			fmt.Printf("Error generating dataset: %v\n", err)
			os.Exit(1)
		}
	}
}

func generateDataset(db *sql.DB) error {

	enc, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		return err
	}

	labels, err := readFileLines(*labelsFile)
	if err != nil {
		return err
	}

	types, err := readFileLines(*typesFile)
	if err != nil {
		return err
	}

	var systemPrompt = PromptMessage{
		Role: "system",
		Content: `
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

	ids, err := readFileLines(*idsFile)
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
		prompt.Messages = append(prompt.Messages, systemPrompt)
		var id int
		var title string
		var description string
		var labels string
		err = rows.Scan(&id, &title, &description, &labels)
		if err != nil {
			return err
		}
		prompt.Messages = append(prompt.Messages, PromptMessage{Role: "user", Content: `
			Issue title\n\n ` + title + `
			Issue description\n\n ` + description + `
		`})

		areaLabels, typeLabels, err := getLabelsFromIssueLabels(labels)
		if err != nil {
			continue
		}

		// do not use examples without labels
		if len(areaLabels) == 0 && len(typeLabels) == 0 {
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
				Role: "system",
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

	finalPromptsJson, err := json.Marshal(finalPrompts)
	if err != nil {
		return err
	}

	//write to out file
	err = os.WriteFile(*outFile, finalPromptsJson, 0644)
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

func getCommand() (string, error) {
	args := flag.Args()
	if len(args) == 0 {
		return "", fmt.Errorf("command is required")
	}

	if !slices.Contains(availableCommands, args[0]) {
		return "", fmt.Errorf("invalid command %s", args[0])
	}

	return args[0], nil
}

func validateFlags() error {
	if *issueDbFile == "" {
		return fmt.Errorf("issueDbFile is required")
	}
	// check that issueDbFile exists
	_, err := os.Stat(*issueDbFile)
	if os.IsNotExist(err) {
		return fmt.Errorf("issueDbFile %s does not exist", *issueDbFile)
	} else if err != nil {
		return err
	}

	if openAiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY env var is required")
	}

	if *idsFile == "" {
		return fmt.Errorf("idsFile is required")
	}

	_, err = os.Stat(*idsFile)
	if os.IsNotExist(err) {
		return fmt.Errorf("idsFile %s does not exist", *idsFile)
	} else if err != nil {
		return err
	}

	return nil
}

func getDb() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", *issueDbFile)
	if err != nil {
		return nil, err
	}

	return db, nil
}

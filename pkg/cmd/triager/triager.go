package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"

	"github.com/google/generative-ai-go/genai"
	"github.com/grafana/auto-triage/pkg/cmd/github"
	"github.com/grafana/auto-triage/pkg/cmd/historian"
	"github.com/grafana/auto-triage/pkg/cmd/vectorizer"
	_ "github.com/mattn/go-sqlite3"
	"github.com/philippgille/chromem-go"
	"google.golang.org/api/option"
)

var geminiKey = os.Getenv("GEMINI_API_KEY")
var geminiModelName = "gemini-1.5-pro"
var embbedModelName = "embedding-001"

var (
	issueDbFile   = flag.String("issuesDb", "github-data.sqlite", "Issue database file")
	updateVectors = flag.Bool("updateVectors", false, "Update vectors")
	vectorDbPath  = flag.String("vectorDb", "vector.db", "Vector database file")
	issueId       = flag.Int("issueId", 0, "Github Issue ID (only the number)")
)

func main() {

	var err error

	flag.Parse()

	err = validateFlags()
	if err != nil {
		fmt.Printf("Error validating flags: %v\n", err)
		os.Exit(1)
	}

	vectorDbInstance := chromem.NewDB()
	ctx := context.Background()
	geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(geminiKey))
	if err != nil {
		log.Fatal(err)
	}
	defer geminiClient.Close()

	err = restoreVectors(vectorDbInstance)
	if err != nil {
		log.Fatal("Error restoring vectors: ", err)
	}

	// init the collection
	collection, err := vectorDbInstance.GetOrCreateCollection("issues", nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Total stored embbedings: %d\n", collection.Count())

	if *updateVectors {
		fmt.Println(":: Updating vectors")
		sqliteDb, err := sql.Open("sqlite3", *issueDbFile)
		if err != nil {
			log.Fatal("Error opening issue database: ", err)
		}
		defer sqliteDb.Close()
		err = vectorizer.VectorizeIssues(
			embbedModelName,
			geminiClient,
			vectorDbInstance,
			sqliteDb,
			func() error { return saveVectors(vectorDbInstance) },
		)
		if err != nil {
			log.Fatal("Error vectorizing issues: ", err)
		}
		fmt.Println(":: Done updating vectors")
	}

	areaLabelsContent, err := os.ReadFile(path.Join("fixtures", "areaLabels.txt"))
	if err != nil {
		log.Fatal("Error reading typeLabels.txt: ", err)
	}
	areaLabels := string(areaLabelsContent)

	typeLabelsContent, err := os.ReadFile(path.Join("fixtures", "typeLabels.txt"))
	if err != nil {
		log.Fatal("Error reading areaLabels.txt: ", err)
	}
	typeLabels := string(typeLabelsContent)

	issueData, err := github.FetchIssueDetails(*issueId)
	if err != nil {
		log.Fatal("Error fetching issue details: ", err)
	}
	fmt.Printf(":: Got issue: %s\n", issueData.Title)

	genModel := geminiClient.GenerativeModel(geminiModelName)
	relatedIssuesContent, err := historian.FindRelevantDocuments(
		geminiClient.EmbeddingModel(embbedModelName),
		genModel,
		collection,
		issueData,
	)
	if err != nil {
		log.Fatal("Error finding relevant documents: ", err)
	}
	if len(relatedIssuesContent) == 0 {
		fmt.Println(":: No relevant documents found. Skipping triage")
		return
	}

	fmt.Printf(":: Found %d relevant issues\n", len(relatedIssuesContent))

	relatedIssuePrompts := ""

	for _, doc := range relatedIssuesContent {
		relatedIssuePrompts += fmt.Sprintf("--##--\n%s\n--##--\n", doc)
	}

	prompt := `

	Based on the following historic issue data:

	---- START HISTORIC DATA ----` +
		relatedIssuePrompts +
		`
	---- END HISTORIC DATA ----

	And the current issue:

	---- START CURRENT ISSUE ---- 
	ID: ` + strconv.Itoa(issueData.Number) + `
	Title: ` + issueData.Title + `
	Description : ` + issueData.Body + `
	---- END CURRENT ISSUE ----

	Categorize the issue:

	In one of the following types:` +
		typeLabels +
		`
	In one of the following areas:` +
		areaLabels +
		``

	genModel.GenerationConfig.ResponseMIMEType = "application/json"

	// system prompt. Has the most weight
	genModel.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(
				`
				You are an expert Grafana issues categorizer.

				You are provided with a list of relevant historic Grafana issues.

				you will categorize the "current" issue based only and only on the historic data.

				The output should be a valid json object with the following fields:

				* id: The id of the current issue
				* areaLabel: The area label of the current issue
				* typeLabel: The type of the current issue
				* summary: The summary of the current issue
        `,
			),
		},
	}

	resp, err := genModel.GenerateContent(context.Background(), genai.Text(prompt))
	if err != nil {
		log.Fatal(err)
	}

	textResponse := getTextContentFromModelContentResponse(resp)
	fmt.Println(textResponse)
}

func saveVectors(db *chromem.DB) error {
	err := db.Export(*vectorDbPath, false, "")
	if err != nil {
		return err
	}
	fmt.Printf("Saved vectors to %s\n", *vectorDbPath)
	return nil
}

func restoreVectors(db *chromem.DB) error {
	_, err := os.Stat(*vectorDbPath)
	if err == nil {
		err = db.Import(*vectorDbPath, "")
		if err != nil {
			return err
		}
		fmt.Printf("Restored vectors from %s\n", *vectorDbPath)
		return nil
	}
	fmt.Printf("No vectors found in %s\n", *vectorDbPath)
	fmt.Printf("error: %v\n", err)
	return nil
}

func validateFlags() error {
	if *updateVectors && *issueDbFile == "" {
		return fmt.Errorf("issueDbFile is required when updateVectors is true")
	}

	if *issueDbFile == "" {
		// check that issueDbFile exists
		_, err := os.Stat(*issueDbFile)
		if os.IsNotExist(err) {
			return fmt.Errorf("issueDbFile %s does not exist", *issueDbFile)
		}
		if err != nil {
			return err
		}
	}
	if issueId == nil {
		return fmt.Errorf("issueUrl is required")
	}

	if geminiKey == "" {
		return fmt.Errorf("GEMINI_KEY is required")
	}

	return nil
}

func getTextContentFromModelContentResponse(modelResponse *genai.GenerateContentResponse) string {
	if len(modelResponse.Candidates) == 0 {
		return ""
	}
	content := modelResponse.Candidates[0].Content
	finalContent := ""
	for _, part := range content.Parts {
		finalContent += fmt.Sprint(part)
	}
	return finalContent
}

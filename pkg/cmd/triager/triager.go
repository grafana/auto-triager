package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"github.com/grafana/auto-triage/pkg/cmd/github"
	"github.com/grafana/auto-triage/pkg/cmd/vectorizer"
	_ "github.com/mattn/go-sqlite3"
	"github.com/philippgille/chromem-go"
	"google.golang.org/api/option"
)

var geminiKey = os.Getenv("GEMINI_API_KEY")

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

	issueData, err := github.FetchIssueDetails(*issueId)
	if err != nil {
		log.Fatal("Error fetching issue details: ", err)
	}
	fmt.Printf(":: Got issue: %s\n", issueData.Title)

	relatedContent, err := findRelevantDocuments(geminiClient, collection, issueData)
	if err != nil {
		log.Fatal("Error finding relevant documents: ", err)
	}
	fmt.Printf(":: Found %d relevant documents\n", len(relatedContent))

	if len(relatedContent) == 0 {
		fmt.Println(":: No relevant documents found. Skipping triage")
		return
	}

	relatedIssuePrompts := ""

	for _, doc := range relatedContent {
		relatedIssuePrompts += fmt.Sprintf("--##--\n%s\n--##--\n", doc)
	}

	promptTemplate := `

	Based on the following historic issue data:

	---- START HISTORIC DATA ----
	%s
	---- END HISTORIC DATA ----

	And the current issue:

	---- START CURRENT ISSUE ---- 
	Title: %s
	Description %s
	---- END CURRENT ISSUE ----

	Categorize the issue as one of the following:

	In one of the following types:

	type/accessibility
  type/angular-2-react
  type/browser-compatibility
  type/bug
  type/build-packaging
  type/chore
  type/ci
  type/cleanup
  type/codegen
  type/community
  type/debt
  type/design
  type/discussion
  type/docs
  type/duplicate
  type/e2e
  type/epic
  type/feature-request
  type/feature-toggle-enable
  type/feature-toggle-removal
  type/performance
  type/poc
  type/project
  type/proposal
  type/question
  type/refactor
  type/regression
  type/roadmap
  type/tech
  type/ux

  In one of the following areas:

  area/admin/user
  area/alerting
  area/alerting-legacy
  area/alerting/compliance
  area/alerting/evaluation
  area/alerting/notifications
  area/alerting/screenshots
  area/annotations
  area/auth
  area/auth/authproxy
  area/auth/ldap
  area/auth/oauth
  area/auth/rbac
  area/auth/saml
  area/auth/serviceaccount
  area/backend
  area/backend/api
  area/backend/config
  area/backend/db
  area/backend/db/migration
  area/backend/db/mysql
  area/backend/db/postgres
  area/backend/db/sql
  area/backend/db/sqlite
  area/backend/logging
  area/backend/plugins
  area/backend/security
  area/backend/user
  area/backend_srv
  area/configuration
  area/dashboard
  area/dashboard/data-links
  area/dashboard/drag-n-drop
  area/dashboard/folders
  area/dashboard/history
  area/dashboard/import
  area/dashboard/links
  area/dashboard/previews
  area/dashboard/rows
  area/dashboard/schemas
  area/dashboard/scripted
  area/dashboard/snapshot
  area/dashboard/templating
  area/dashboard/timerange
  area/dashboard/timezone
  area/dashboards/panel
  area/dashgpt
  area/data/export
  area/dataframe
  area/dataplane
  area/datasource
  area/datasource/backend
  area/datasource/frontend
  area/datasource/proxy
  area/dataviz
  area/developer-portal
  area/devenv
  area/docker
  area/drag-n-drop
  area/editor
  area/explore
  area/expressions
  area/field/overrides
  area/frontend
  area/frontend/code-editor
  area/frontend/library-panels
  area/frontend/library-variables
  area/frontend/login
  area/generativeAI
  area/glue
  area/grafana-cli
  area/grafana.com
  area/grafana/data
  area/grafana/e2e
  area/grafana/runtime
  area/grafana/toolkit
  area/grafana/ui
  area/grpc-server
  area/http-server
  area/image-rendering
  area/imagestore
  area/instrumentation
  area/internationalization
  area/kindsys
  area/legend
  area/libraries
  area/live
  area/logs
  area/mailing
  area/manage-dashboards
  area/meta-analytics
  area/mobile-support
  area/navigation
  area/panel-chrome
  area/panel/alertlist
  area/panel/annotation-list
  area/panel/barchart
  area/panel/bargauge
  area/panel/candlestick
  area/panel/canvas
  area/panel/common
  area/panel/dashboard-list
  area/panel/data
  area/panel/datagrid
  area/panel/edit
  area/panel/flame-graph
  area/panel/gauge
  area/panel/geomap
  area/panel/graph
  area/panel/heatmap
  area/panel/histogram
  area/panel/icon
  area/panel/infrastructure
  area/panel/logs
  area/panel/news
  area/panel/node-graph
  area/panel/piechart
  area/panel/singlestat
  area/panel/stat
  area/panel/state-timeline
  area/panel/status-history
  area/panel/support
  area/panel/table
  area/panel/text
  area/panel/timeseries
  area/panel/traceview
  area/panel/trend
  area/panel/xychart
  area/permissions
  area/playlist
  area/plugin-extensions
  area/plugins
  area/plugins-catalog
  area/plugins/app
  area/plugins/sandbox
  area/profiling
  area/provisioning
  area/public-dashboards
  area/query-editor
  area/recorded-queries
  area/resourcepicker
  area/reusable-queries
  area/routing
  area/scenes
  area/schema
  area/search
  area/security
  area/short-url
  area/shortcuts
  area/stack
  area/storybook
  area/streaming
  area/teams
  area/templating/repeating
  area/text-formatting
  area/thema
  area/tooltip
  area/tracing
  area/transformations
  area/ui/theme
  area/units
  area/ux
  area/value-mapping


	`

	prompt := fmt.Sprintf(promptTemplate, relatedIssuePrompts)

	model := geminiClient.GenerativeModel("gemini-1.5-pro")
	model.GenerationConfig.ResponseMIMEType = "application/json"

	// system prompt. Has the most weight
	model.SystemInstruction = &genai.Content{
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

	resp, err := model.GenerateContent(context.Background(), genai.Text(prompt))
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

func findRelevantDocuments(
	geminiClient *genai.Client,
	collection *chromem.Collection,
	issue github.Issue,
) ([]string, error) {
	embbedModel := geminiClient.EmbeddingModel("embedding-001")

	embedding, err := embbedModel.EmbedContentWithTitle(
		context.Background(),
		issue.Title,
		genai.Text(issue.Body),
	)
	if err != nil {
		return nil, err
	}

	documents, err := collection.QueryEmbedding(
		context.Background(),
		embedding.Embedding.Values,
		50,
		nil,
		nil,
	)

	if err != nil {
		return nil, err
	}

	results := make([]string, len(documents))
	for i, doc := range documents {
		results[i] = doc.Content
	}

	return results, nil

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

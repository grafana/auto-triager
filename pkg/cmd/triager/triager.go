package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"github.com/grafana/auto-triage/pkg/cmd/vectorizer"
	_ "github.com/mattn/go-sqlite3"
	"github.com/philippgille/chromem-go"
	"google.golang.org/api/option"
)

var geminiKey = os.Getenv("GEMINI_API_KEY")

var (
	issueDbFile      = flag.String("issuesDb", "github-data.sqlite", "Issue database file")
	updateVectors    = flag.Bool("updateVectors", false, "Update vectors")
	vectorDbPath     = flag.String("vectorDb", "vector.db", "Vector database file")
	issueTitle       = flag.String("issueTitle", "", "Issue Title")
	issueDescription = flag.String("issueDescription", "", "Issue Description")
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
	em := geminiClient.EmbeddingModel("embedding-001")
	embedFunc := func(ctx context.Context, text string) ([]float32, error) {
		res, err := em.EmbedContent(ctx, genai.Text(text))
		if err != nil {
			return nil, err
		}
		return res.Embedding.Values, nil
	}
	collection, err := vectorDbInstance.GetOrCreateCollection("issues", nil, embedFunc)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Total stored embbedings: %d\n", collection.Count())

	if *updateVectors {
		sqliteDb, err := sql.Open("sqlite3", *issueDbFile)
		if err != nil {
			log.Fatal("Error opening issue database: ", err)
		}
		defer sqliteDb.Close()
		err = vectorizer.VectorizeIssues(collection, issueDbFile)
		if err != nil {
			log.Fatal("Error vectorizing issues: ", err)
		}

		fmt.Println("Done updating vectors")
		err = vectorDbInstance.Export(*vectorDbPath, false, "")
		if err != nil {
			log.Fatal("Error exporting vectors: ", err)
		}
		fmt.Println("Done exporting vectors")
		os.Exit(0)
	}
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
	if issueTitle == nil || *issueTitle == "" {
		return fmt.Errorf("issueTitle is required")
	}
	if issueDescription == nil || *issueDescription == "" {
		return fmt.Errorf("issueDescription is required")
	}

	if geminiKey == "" {
		return fmt.Errorf("GEMINI_KEY is required")
	}

	return nil
}

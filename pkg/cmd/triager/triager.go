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
var ghKey = os.Getenv("GH_TOKEN")

var (
	issueDbFile   = flag.String("issuesDb", "github-data.sqlite", "Issue database file")
	updateVectors = flag.Bool("updateVectors", false, "Update vectors")
	vectorDbPath  = flag.String("vectorDb", "vector.db", "Vector database file")
	issueUrl      = flag.String("issueUrl", "", "Github Issue URL")
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

		fmt.Println("Done updating vectors")
		err = saveVectors(vectorDbInstance)
		if err != nil {
			log.Fatal("Error exporting vectors: ", err)
		}
		fmt.Println("Done exporting vectors")
		os.Exit(0)
	}
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
	if issueUrl == nil {
		return fmt.Errorf("issueUrl is required")
	}

	if geminiKey == "" {
		return fmt.Errorf("GEMINI_KEY is required")
	}

	return nil
}

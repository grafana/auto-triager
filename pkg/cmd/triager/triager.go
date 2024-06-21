package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/grafana/auto-triage/pkg/commontypes"
	_ "github.com/mattn/go-sqlite3"
	"github.com/philippgille/chromem-go"
)

var (
	issueDbFile      = flag.String("issuesDb", "github-data.sqlite", "Issue database file")
	updateVectors    = flag.Bool("updateVectors", false, "Update vectors")
	vectorDb         = flag.String("vectorDb", "vector.db", "Vector database file")
	issueTitle       = flag.String("issueTitle", "", "Issue Title")
	issueDescription = flag.String("issueDescription", "", "Issue Description")
)

func main() {

	flag.Parse()

	err := validateFlags()
	if err != nil {
		fmt.Printf("Error validating flags: %v\n", err)
		os.Exit(1)
	}

	db := chromem.NewDB()

	if !*updateVectors {
		err = restoreVecots(db)
		if err != nil {
			fmt.Printf("Error restoring vectors: %v\n", err)
			os.Exit(1)
		}
	}

	if *updateVectors {
		err = updateVectorsDb(db, issueDbFile)
		if err != nil {
			fmt.Printf("Error updating vectors: %v\n", err)
			os.Exit(1)
		}
	}
}

func updateVectorsDb(db *chromem.DB, issueDbFile *string) error {
	// open sqlite db
	sqliteDb, err := sql.Open("sqlite3", *issueDbFile)
	if err != nil {
		return err
	}

	defer sqliteDb.Close()

	row := sqliteDb.QueryRow("SELECT count(*) as count FROM issues")

	var count int
	err = row.Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no issues found in %s", *issueDbFile)
	}

	fmt.Printf("Found %d issues in %s\n", count, *issueDbFile)

	fmt.Printf("Updating vectors in %s\n", *vectorDb)
	err = db.DeleteCollection("issues")
	if err != nil {
		return err
	}

	collection, err := db.CreateCollection("issues", nil, nil)
	if err != nil {
		return err
	}

	rows, err := sqliteDb.Query("SELECT id, title, description, labels, raw FROM issues")
	if err != nil {
		return err
	}

	var documents []chromem.Document
	count = 0

	for rows.Next() {
		var id int
		var title string
		var description string
		var labels string
		var raw string
		err = rows.Scan(&id, &title, &description, &labels, &raw)
		if err != nil {
			return err
		}
		if len(labels) == 0 {
			continue
		}
		var parsedLabels []commontypes.Label
		err = json.Unmarshal([]byte(labels), &parsedLabels)
		if err != nil {
			return err
		}

		labelsString := ""
		for _, label := range parsedLabels {
			labelsString += label.Name + ", "
		}

		fmt.Println("  - Saving issue in vector db:", id)

		documents = append(documents, chromem.Document{
			ID: fmt.Sprintf("%d", id),
			Content: `# ` + title +
				`## labels \n` + labelsString +
				`## Description \n` + description + `
			`,
		})

		count++
		if count%300 == 0 {
			fmt.Printf("  - Adding %d documents to vector db\n", count)
			err = collection.AddDocuments(context.Background(), documents, runtime.NumCPU())
			if err != nil {
				return err
			}
			count = 0
			documents = []chromem.Document{}
		}
	}
	if count > 0 {
		fmt.Printf("  - Adding %d documents to vector db\n", count)
		err = collection.AddDocuments(context.Background(), documents, runtime.NumCPU())
		if err != nil {
			return err
		}
	}

	fmt.Println("  - Done adding documents to vector db")

	if err != nil {
		return err
	}

	err = db.Export(*vectorDb, true, "super-strong-key")
	if err != nil {
		return err
	}

	return nil
}

func restoreVecots(db *chromem.DB) error {
	_, err := os.Stat(*vectorDb)
	if err == nil {
		err = db.Import(*vectorDb, "super-strong-key")
		if err != nil {
			return err
		}
	}
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
	fmt.Println("Title: ", *issueTitle)
	if issueTitle == nil || *issueTitle == "" {
		return fmt.Errorf("issueTitle is required")
	}
	if issueDescription == nil || *issueDescription == "" {
		return fmt.Errorf("issueDescription is required")
	}

	return nil
}

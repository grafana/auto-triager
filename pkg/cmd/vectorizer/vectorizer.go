package vectorizer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/grafana/auto-triage/pkg/commontypes"
	"github.com/philippgille/chromem-go"

	//sqlite driver
	_ "github.com/mattn/go-sqlite3"
)

const numWorkers = 10
const batchSize = 100

type Task struct {
	ID          int
	Title       string
	Description string
	Labels      string
	// Add other fields as necessary.
}

type WorkerOps struct {
	collection *chromem.Collection
	sqliteDB   *sql.DB
}

// Worker function to process tasks.
func worker(id int, tasks <-chan Task, wg *sync.WaitGroup, ops WorkerOps) {
	ctx := context.Background()
	defer wg.Done()
	for task := range tasks {
		// fmt.Printf("Worker %d processing issue %d\n", id, task.ID)

		content := `
			  Title: ` + task.Title + `
			  Description: ` + task.Description + `
			  Labels: ` + task.Labels + `
		`
		// store it inside the collection
		err := ops.collection.AddDocument(ctx, chromem.Document{
			ID:      strconv.Itoa(task.ID),
			Content: content,
			// Embedding: res.Embedding.Values,
			Metadata: map[string]string{
				"idKey":  strconv.Itoa(id),
				"labels": task.Labels,
			},
		})

		fmt.Printf("Got embbedings for issue %d\n", task.ID)

		if err != nil {
			fmt.Printf("Error storing embedding for issue %d '%s': %v\n", task.ID, task.Title, err)
			return
		}
	}
	fmt.Printf("Worker %d done\n", id)
}

func VectorizeIssues(
	collection *chromem.Collection,
	issueDbFile *string,
	saveDb func() error,
) error {
	// open sqlite db
	sqliteDb, err := sql.Open("sqlite3", *issueDbFile)
	if err != nil {
		return err
	}

	defer sqliteDb.Close()

	batchCount := 0
	for {
		batchCount++
		row := sqliteDb.QueryRow("SELECT count(*) as count FROM issues WHERE processed = 0")

		var count int
		err = row.Scan(&count)
		if err != nil {
			fmt.Printf("Error scanning row: %v\n", err)
			return err
		}
		if count == 0 {
			fmt.Printf("No more issues to process\n")
			break
		}

		tasks := make(chan Task, numWorkers)
		var wg sync.WaitGroup

		// Start worker goroutines.
		for i := 1; i <= numWorkers; i++ {
			wg.Add(1)
			go worker(i, tasks, &wg, WorkerOps{collection, sqliteDb})
		}

		fmt.Printf("Found %d issues to process\n", count)
		fmt.Printf("Batch %d\n", batchCount)

		rows, err := sqliteDb.Query(
			"SELECT id, title, description, labels, raw FROM issues WHERE processed = 0 ORDER BY id ASC LIMIT ?",
			batchSize,
		)

		if err != nil {
			fmt.Printf("Error querying issues: %v\n", err)
			return err
		}

		count = 0
		ids := []string{}
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

			ids = append(ids, strconv.Itoa(id))

			tasks <- Task{
				ID:          id,
				Title:       title,
				Description: description,
				Labels:      parseLabels(labels),
			}

		}
		rows.Close()
		close(tasks)
		fmt.Printf("Waiting for workers to finish processing batch %d\n", batchCount)
		wg.Wait()
		fmt.Printf("Marking %d issues as processed\n", len(ids))
		// mark the issues as processed
		_, err = sqliteDb.Exec(
			"UPDATE issues SET processed =  1 WHERE id IN (" + strings.Join(ids, ",") + ")",
		)
		if err != nil {
			fmt.Printf("Error marking issues as processed: %v\n", err)
		}

		err = saveDb()
		if err != nil {
			return err
		}
		fmt.Printf("batch %d processed\n", batchCount)
	}

	return nil
}

func parseLabels(labels string) string {
	if len(labels) == 0 {
		return ""
	}
	var parsedLabels []commontypes.Label
	err := json.Unmarshal([]byte(labels), &parsedLabels)
	if err != nil {
		return ""
	}

	labelsString := ""
	for _, label := range parsedLabels {
		labelsString += label.Name + ", "
	}

	return labelsString

}

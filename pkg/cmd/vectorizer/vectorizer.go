package vectorizer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/grafana/auto-triage/pkg/commontypes"
	"github.com/philippgille/chromem-go"

	//sqlite driver
	_ "github.com/mattn/go-sqlite3"
)

const numWorkers = 5

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
		fmt.Printf("Worker %d processing issue %d\n", id, task.ID)

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

		// mark the issue as processed
		_, err = ops.sqliteDB.Exec("UPDATE issues SET processed = 1 WHERE id = ?", task.ID)
		if err != nil {
			fmt.Printf("Error marking issue %d as processed: %v\n", task.ID, err)
			return
		}

		fmt.Printf("Worker %d processed issue %d\n", id, task.ID)

	}
}

func VectorizeIssues(collection *chromem.Collection, issueDbFile *string) error {
	// open sqlite db
	sqliteDb, err := sql.Open("sqlite3", *issueDbFile)
	if err != nil {
		return err
	}

	defer sqliteDb.Close()

	row := sqliteDb.QueryRow("SELECT count(*) as count FROM issues WHERE processed = 0")

	var count int
	err = row.Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no issues found in %s", *issueDbFile)
	}

	fmt.Printf("Found %d issues in %s\n", count, *issueDbFile)

	count = collection.Count()
	fmt.Printf("Found %d vectors in collection\n", count)

	rows, err := sqliteDb.Query(
		"SELECT id, title, description, labels, raw FROM issues WHERE processed = 0",
	)
	if err != nil {
		return err
	}

	tasks := make(chan Task, numWorkers)
	var wg sync.WaitGroup

	// Start worker goroutines.
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(i, tasks, &wg, WorkerOps{collection, sqliteDb})
	}

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

		tasks <- Task{
			ID:          id,
			Title:       title,
			Description: description,
			Labels:      parseLabels(labels),
		}

	}

	close(tasks)
	wg.Wait()
	count = collection.Count()
	fmt.Printf("Found %d vectors in collection (after)\n", count)

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

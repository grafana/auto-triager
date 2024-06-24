package vectorizer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/grafana/auto-triage/pkg/commontypes"
	"github.com/philippgille/chromem-go"

	//sqlite driver
	_ "github.com/mattn/go-sqlite3"
)

const batchSize = 100

type BatchItem struct {
	ID        int
	Content   string
	Labels    string
	Embedding []float32
}

func VectorizeIssues(
	geminiClient *genai.Client,
	vectorDb *chromem.DB,
	sqliteDb *sql.DB,
	saveDb func() error,
) error {

	var err error

	ctx := context.Background()

	embbedModel := geminiClient.EmbeddingModel("embedding-001")

	collection, err := vectorDb.GetOrCreateCollection("issues", nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	row := sqliteDb.QueryRow("SELECT count(*) as totalCount FROM issues WHERE processed = 0")

	var totalCount int
	err = row.Scan(&totalCount)
	if err != nil {
		fmt.Printf("Error scanning row: %v\n", err)
		return err
	}
	if totalCount == 0 {
		fmt.Printf(":: No issues to process. Skipping update of vectors\n")
		return nil
	}

	totalBatches := (totalCount + batchSize - 1) / batchSize

	fmt.Printf(":: Total issues to process: %d\n", totalCount)
	fmt.Printf(":: Will process in batches of %d\n", batchSize)
	fmt.Printf(":: Total batches: %d\n", totalBatches)

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

		fmt.Printf("    -> Found %d issues to process\n", count)
		fmt.Printf("Batch %d of %d\n", batchCount, totalBatches)

		rows, err := sqliteDb.Query(
			"SELECT id, title, description, labels, raw FROM issues WHERE processed = 0 ORDER BY id ASC LIMIT ?",
			batchSize,
		)

		if err != nil {
			fmt.Printf("Error querying issues: %v\n", err)
			return err
		}

		batchEmbed := embbedModel.NewBatch()

		batchItems := make([]BatchItem, batchSize)

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
			content := `
						  Title: ` + title + `
						  Description: ` + description + `
						  Labels: ` + parseLabels(labels) + `
					`

			batchEmbed.AddContentWithTitle(title, genai.Text(content))
			ids = append(ids, strconv.Itoa(id))

			batchItems[count] = BatchItem{
				ID:        id,
				Content:   content,
				Labels:    labels,
				Embedding: nil,
			}
			count++
		}
		rows.Close()

		fmt.Printf("    -> Embedding batch %d with %d items\n", batchCount, count)
		embedRes, err := embbedModel.BatchEmbedContents(ctx, batchEmbed)
		timeSinceLast := time.Now()
		if err != nil {
			fmt.Printf("Error embedding batch: %v\n", err)
			return err
		}

		fmt.Printf(
			"    -> Got embbedings for batch %d. Total embbedings %d\n",
			batchCount,
			len(embedRes.Embeddings),
		)

		// google returns embbedings in the same order as the input
		for i, geminiEmbed := range embedRes.Embeddings {
			// items[i].Embedding = item.Values

			err = collection.AddDocument(ctx, chromem.Document{
				ID:        strconv.Itoa(batchItems[i].ID),
				Content:   batchItems[i].Content,
				Embedding: geminiEmbed.Values,
				Metadata: map[string]string{
					"idKey":  strconv.Itoa(batchItems[i].ID),
					"labels": batchItems[i].Labels,
				},
			})

			if err != nil {
				fmt.Printf(
					"Error storing embedding for issue %d : %v\n",
					batchItems[i].ID,
					err,
				)
				return err
			}
		}

		err = saveDb()
		if err != nil {
			return err
		}

		// mark the issues as processed
		_, err = sqliteDb.Exec(
			"UPDATE issues SET processed =  1 WHERE id IN (" + strings.Join(ids, ",") + ")",
		)
		if err != nil {
			fmt.Printf("Error marking issues as processed: %v\n", err)
		}

		fmt.Printf("    -> Batch %d of %d processed\n", batchCount, totalBatches)
		// make sure 1.1 seconds passed since the start of this batch, sleep the rest
		leftTime := time.Until(timeSinceLast.Add(time.Second * 1))
		if leftTime > 0 {
			fmt.Printf("    -> Sleeping for %v\n", leftTime)
			time.Sleep(leftTime)
		}
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

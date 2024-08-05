package historian

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/grafana/auto-triage/pkg/github"
	"github.com/philippgille/chromem-go"
)

const batchingSize = 150

// max tokens 900k
const maxTokens int32 = 900000

func FindRelevantDocuments(
	embbedModel *genai.EmbeddingModel,
	genModel *genai.GenerativeModel,
	collection *chromem.Collection,
	issue github.Issue,
) ([]string, error) {

	embedding, err := embbedModel.EmbedContentWithTitle(
		context.Background(),
		issue.Title,
		genai.Text(issue.Body),
	)
	if err != nil {
		return nil, err
	}

	// we get 1000 relevant documents
	documents, err := collection.QueryEmbedding(
		context.Background(),
		embedding.Embedding.Values,
		1000,
		nil,
		nil,
	)
	if err != nil {
		return nil, err
	}

	content := make([]string, len(documents))
	for i, doc := range documents {
		content[i] = doc.Content
	}

	isValid := true
	current := 0
	currentResults := []string{}
	results := []string{}

	for isValid {
		nextSize := current + batchingSize
		if nextSize >= len(content) {
			nextSize = len(content)
			isValid = false
		}
		// append 50 from content to results
		currentResults = append(currentResults, content[current:nextSize]...)
		textContent := strings.Join(currentResults, "---##$$##---")

		tokensCount, err := genModel.CountTokens(context.Background(), genai.Text(textContent))
		if err != nil {
			break
		}
		if tokensCount.TotalTokens >= maxTokens {
			fmt.Printf(
				"     :- Reached %d tokens. Max tokens set to %d. Breaking\n",
				tokensCount.TotalTokens,
				maxTokens,
			)
			break
		}

		fmt.Printf(
			"     :- Current tokens count: %d. Adding more documents...\n",
			tokensCount.TotalTokens,
		)
		results = currentResults
		current = current + batchingSize
	}

	return results, nil

}

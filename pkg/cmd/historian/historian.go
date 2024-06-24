package historian

import (
	"context"

	"github.com/google/generative-ai-go/genai"
	"github.com/grafana/auto-triage/pkg/cmd/github"
	"github.com/philippgille/chromem-go"
)

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

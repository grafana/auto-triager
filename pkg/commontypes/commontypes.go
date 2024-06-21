package commontypes

import (
	"encoding/json"
	"time"
)

type Issue struct {
	Number      int         `json:"number"`
	Title       string      `json:"title"`
	Description string      `json:"body"`
	PullRequest PullRequest `json:"pull_request"`
	Labels      []Label     `json:"labels"`
	Raw         json.RawMessage
}

type PullRequest struct {
	URL      string    `json:"url"`
	MergedAt time.Time `json:"merged_at"`
}

type Label struct {
	Name string `json:"name"`
}

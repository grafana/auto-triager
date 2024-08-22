package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/grafana/auto-triage/pkg/logme"
)

type Issue struct {
	URL                   string     `json:"url"`
	RepositoryURL         string     `json:"repository_url"`
	LabelsURL             string     `json:"labels_url"`
	Labels                []Label    `json:"labels"`
	CommentsURL           string     `json:"comments_url"`
	EventsURL             string     `json:"events_url"`
	HTMLURL               string     `json:"html_url"`
	ID                    int64      `json:"id"`
	NodeID                string     `json:"node_id"`
	Number                int        `json:"number"`
	Title                 string     `json:"title"`
	User                  User       `json:"user"`
	State                 string     `json:"state"`
	Locked                bool       `json:"locked"`
	Assignee              *User      `json:"assignee"`
	Comments              int        `json:"comments"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	ClosedAt              *time.Time `json:"closed_at"`
	AuthorAssociation     string     `json:"author_association"`
	ActiveLockReason      *string    `json:"active_lock_reason"`
	Body                  string     `json:"body"`
	ClosedBy              *User      `json:"closed_by"`
	Reactions             Reactions  `json:"reactions"`
	TimelineURL           string     `json:"timeline_url"`
	PerformedViaGithubApp *string    `json:"performed_via_github_app"`
	StateReason           *string    `json:"state_reason"`
}

type Label struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Url   string `json:"url"`
}

type User struct {
	Login             string `json:"login"`
	ID                int64  `json:"id"`
	NodeID            string `json:"node_id"`
	AvatarURL         string `json:"avatar_url"`
	GravatarID        string `json:"gravatar_id"`
	URL               string `json:"url"`
	HTMLURL           string `json:"html_url"`
	FollowersURL      string `json:"followers_url"`
	FollowingURL      string `json:"following_url"`
	GistsURL          string `json:"gists_url"`
	StarredURL        string `json:"starred_url"`
	SubscriptionsURL  string `json:"subscriptions_url"`
	OrganizationsURL  string `json:"organizations_url"`
	ReposURL          string `json:"repos_url"`
	EventsURL         string `json:"events_url"`
	ReceivedEventsURL string `json:"received_events_url"`
	Type              string `json:"type"`
	SiteAdmin         bool   `json:"site_admin"`
}

type Reactions struct {
	URL        string `json:"url"`
	TotalCount int    `json:"total_count"`
	PlusOne    int    `json:"+1"`
	MinusOne   int    `json:"-1"`
	Laugh      int    `json:"laugh"`
	Hooray     int    `json:"hooray"`
	Confused   int    `json:"confused"`
	Heart      int    `json:"heart"`
	Rocket     int    `json:"rocket"`
	Eyes       int    `json:"eyes"`
}

var githubToken = os.Getenv("GH_TOKEN")

func FetchIssueDetails(issueId int, repo string) (Issue, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://api.github.com/repos/%s/issues/%d", repo, issueId),
		nil,
	)

	if err != nil {
		return Issue{}, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("token %s", githubToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Issue{}, err
	}

	issue := Issue{}
	err = json.NewDecoder(resp.Body).Decode(&issue)
	if err != nil {
		return Issue{}, err
	}

	return issue, nil
}

func FetchGrafanaIssueDetails(issueId int) (Issue, error) {
	return FetchIssueDetails(issueId, "grafana/grafana")
}

func PublishIssueToRepo(repo string, issue Issue, labels []string) (Issue, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues", repo)

	payload, err := json.Marshal(map[string]interface{}{
		"title":  issue.Title,
		"body":   issue.Body,
		"labels": labels,
	})

	logme.DebugF("Payload: %s\n", payload)
	logme.DebugF("URL: %s\n", url)

	if err != nil {
		return Issue{}, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return Issue{}, err
	}

	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Issue{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return Issue{}, fmt.Errorf("Error creating issue. Status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Issue{}, err
	}

	var createdIssue Issue
	if err := json.Unmarshal(body, &createdIssue); err != nil {
		return Issue{}, err
	}

	return createdIssue, nil
}

func AddLabelsToIssue(repo string, issueId int, labels []string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/labels", repo, issueId)

	payload, err := json.Marshal(map[string]interface{}{
		"labels": labels,
	})

	logme.DebugF("Payload: %s\n", payload)
	logme.DebugF("URL: %s\n", url)

	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error creating issue. Status code: %d", resp.StatusCode)
	}

	return nil
}

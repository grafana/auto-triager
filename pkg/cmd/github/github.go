package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type Issue struct {
	URL                   string     `json:"url"`
	RepositoryURL         string     `json:"repository_url"`
	LabelsURL             string     `json:"labels_url"`
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

func FetchIssueDetails(issueId int) (Issue, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://api.github.com/repos/grafana/grafana/issues/%d", issueId),
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

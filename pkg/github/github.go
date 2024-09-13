package github

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
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

type GithubSearchIssueResult struct {
	Items []Issue `json:"items"`
}

func FetchIssueDetails(issueId int, repo string) (Issue, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://api.github.com/repos/%s/issues/%d", repo, issueId),
		nil,
	)

	if err != nil {
		return Issue{}, err
	}

	githubToken := os.Getenv("GH_TOKEN")

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

	githubToken := os.Getenv("GH_TOKEN")

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

	githubToken := os.Getenv("GH_TOKEN")

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

func GetIssuesByFilter(filter string, perPage int, page int) ([]Issue, error) {
	var url = fmt.Sprintf("https://api.github.com/search/issues?q=%s&per_page=%d&page=%d", filter, perPage, page)
	fmt.Printf("        --> URL: %s\n", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	githubToken := os.Getenv("GH_TOKEN")

	fmt.Printf("URL: %s\n", req.URL.String())

	req.Header.Set("Authorization", "token "+githubToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var issues GithubSearchIssueResult
	err = json.Unmarshal(body, &issues)
	if err != nil {
		return nil, err
	}

	return issues.Items, nil
}

func AssignProjectToIssue(issueNodeId string, projecNodeId string) error {
	query := fmt.Sprintf(`
        mutation {
            addProjectV2ItemById(input: {projectId: "%s", contentId: "%s"}) {
                item {
                    id
                }
            }
        }`, projecNodeId, issueNodeId)

	graphqlQuery := map[string]string{"query": query}
	queryJson, err := json.Marshal(graphqlQuery)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		"POST",
		"https://api.github.com/graphql",
		bytes.NewBuffer(queryJson),
	)
	if err != nil {
		return err
	}
	githubToken := os.Getenv("GH_TOKEN")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+githubToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// dump body response
	// fmt.Printf("assign Response body: %s\n", string(body))

	// Parsing response
	var result struct {
		Data struct {
			AddProjectV2ItemById struct {
				Item struct {
					ID string `json:"id"`
				} `json:"item"`
			} `json:"addProjectV2ItemById"`
		} `json:"data"`
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	if len(result.Errors) > 0 {
		return fmt.Errorf("error: %s", result.Errors[0].Message)
	}

	return nil
}

func GetProjectNodeId(org string, projectId int) (string, error) {
	query := fmt.Sprintf(`
        {
            organization(login: "%s") {
                projectV2(number: %d) {
                    id
                }
            }
        }`, org, projectId)

	graphqlQuery := map[string]string{"query": query}
	queryJson, err := json.Marshal(graphqlQuery)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(
		"POST",
		"https://api.github.com/graphql",
		bytes.NewBuffer(queryJson),
	)
	if err != nil {
		return "", err
	}
	githubToken := os.Getenv("GH_TOKEN")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+githubToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// dump body
	// fmt.Printf("Response body: %s\n", string(body))

	// Parsing response
	var result struct {
		Data struct {
			Organization struct {
				ProjectV2 struct {
					ID string `json:"id"`
				} `json:"projectV2"`
			} `json:"organization"`
		} `json:"data"`
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}

	if len(result.Errors) > 0 {
		return "", fmt.Errorf("error: %s", result.Errors[0].Message)
	}

	if result.Data.Organization.ProjectV2.ID == "" {
		return "", fmt.Errorf("project id is empty. Check if the project exists")
	}

	return result.Data.Organization.ProjectV2.ID, nil
}

func GenerateJWT(appID int64, pemPath string) (string, error) {
	privateKey, err := loadPEMKey(pemPath)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(time.Minute * 10).Unix(),
		"iss": appID,
	})

	return token.SignedString(privateKey)
}

// loadPEMKey loads and parses the PEM private key
func loadPEMKey(pemPath string) (*rsa.PrivateKey, error) {
	keyBytes, err := os.ReadFile(pemPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// GetInstallationToken exchanges the JWT for an installation token
func GetInstallationToken(appID int64, pemPath string, installationID int64) (string, error) {
	jwtToken, err := GenerateJWT(appID, pemPath)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID),
		nil,
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to generate installation token, status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return extractTokenFromBody(string(body)), nil
}

// extractTokenFromBody is a simple parser to extract the token from the response (expected as JSON)
func extractTokenFromBody(body string) string {
	// This is where you would parse the JSON body to get the token.
	// For simplicity, use a proper JSON decoding function instead if needed.
	if idx := strings.Index(body, `"token":`); idx != -1 {
		start := idx + len(`"token":`) + 1
		end := strings.Index(body[start:], `"`) + start
		return body[start:end]
	}
	return ""
}

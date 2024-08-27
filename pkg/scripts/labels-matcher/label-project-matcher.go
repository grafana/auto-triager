package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
)

type Response struct {
	Data Data `json:"data"`
}

type Data struct {
	Repository Repository `json:"repository"`
}

type Repository struct {
	Issues Issues `json:"issues"`
}

type Issues struct {
	Edges []IssueEdge `json:"edges"`
}

type IssueEdge struct {
	Node IssueNode `json:"node"`
}

type IssueNode struct {
	Title        string       `json:"title"`
	Number       int          `json:"number"`
	ProjectCards ProjectCards `json:"projectCards"`
}

type ProjectCards struct {
	Edges []ProjectCardEdge `json:"edges"`
}

type ProjectCardEdge struct {
	Node ProjectCardNode `json:"node"`
}

type ProjectCardNode struct {
	Project Project `json:"project"`
}

type Project struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

var label = flag.String("label", "", "Label to search for")

const githubApiUrl = "https://api.github.com/graphql"

func main() {

	flag.Parse()

	if *label == "" {
		fmt.Println("Please provide a label to search for.")
		os.Exit(1)

	}

	// You need to replace this token with your own GitHub personal access token
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		fmt.Println(
			"Please set your GITHUB_TOKEN environment variable with your GitHub personal access token.",
		)
		return
	}

	query := `
	{
  repository(owner: "grafana", name: "grafana") {
    issues(labels: "` + *label + `", first: 100) {
      edges {
        node {
          title
          number
          projectCards(first: 100) {
            edges {
              node {
                project {
                  name
                  url
                }
              }
            }
          }
        }
      }
    }
  }
}
    `

	reqBody := map[string]string{
		"query": query,
	}

	jsonValue, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", githubApiUrl, bytes.NewBuffer(jsonValue))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Non-ok HTTP status:", resp.StatusCode)
		fmt.Println(string(body))
		return
	}

	// Parse JSON Response
	var result Response
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("Error parsing JSON response:", err)
		return
	}

	type DistinctProjectCard struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		Counter int
	}

	projectCards := make(map[string]*DistinctProjectCard)

	// find issues with a project card with project associated and non empty url
	for _, issue := range result.Data.Repository.Issues.Edges {
		for _, projectCard := range issue.Node.ProjectCards.Edges {
			if projectCard.Node.Project.URL != "" {
				// insert distinct project cards
				if _, ok := projectCards[projectCard.Node.Project.URL]; !ok {
					projectCards[projectCard.Node.Project.URL] = &DistinctProjectCard{
						Name:    projectCard.Node.Project.Name,
						URL:     projectCard.Node.Project.URL,
						Counter: 1,
					}
				} else {
					card := projectCards[projectCard.Node.Project.URL]
					card.Counter++
				}
			}
		}
	}

	cards := make([]*DistinctProjectCard, 0, len(projectCards))
	for _, card := range projectCards {
		cards = append(cards, card)
	}

	// Sort the slice by the Counter field
	sort.Slice(cards, func(i, j int) bool {
		return cards[i].Counter > cards[j].Counter // Ascending order
	})

	fmt.Printf("\nThe following projects have the most issues with the label %s:\n\n", *label)

	// Display sorted cards
	for _, card := range cards {
		fmt.Printf("Name: %s, URL: %s, Counter: %d\n", card.Name, card.URL, card.Counter)
	}

}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/grafana/auto-triage/pkg/prettyprint"
)

const query = `
query {
  repository(owner: "grafana", name: "grafana") {
    issues(last: 100, filterBy: {}, states: [CLOSED]) {
      edges {
        node {
          title
          url
          number
          labels(first: 1) {
            edges {
              node {
                name
              }
            }
          }
        }
      }
    }
  }
}`

type Response struct {
	Data struct {
		Repository struct {
			Issues struct {
				Edges []struct {
					Node struct {
						Title  string `json:"title"`
						URL    string `json:"url"`
						Number int    `json:"number"`
						Labels struct {
							Edges []struct {
								Node struct {
									Name string `json:"name"`
								} `json:"node"`
							} `json:"edges"`
						} `json:"labels"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"issues"`
		} `json:"repository"`
	} `json:"data"`
}

func main() {
	fmt.Println("Auto Triage")
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		fmt.Println("GH_TOKEN environment variable not set")
		return
	}

	jsonData := map[string]string{
		"query": query,
	}
	jsonValue, _ := json.Marshal(jsonData)
	request, _ := http.NewRequest(
		"POST",
		"https://api.github.com/graphql",
		bytes.NewBuffer(jsonValue),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("Error unmarshaling response:", err)
		return
	}

	prettyprint.Print(response)

}

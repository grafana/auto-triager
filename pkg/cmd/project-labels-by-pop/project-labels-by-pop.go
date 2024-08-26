package main

// go run project-labels-by-pop.go
// this will fetch all the labels from grafana/grafana
// ordered by the number of issues with that label
// then prints them to stdout

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

func main() {
	err := playwright.Install()
	if err != nil {
		log.Fatalf("could not install playwright: %v", err)
	}

	currentPage := 1
	url := "https://github.com/grafana/grafana/labels?sort=count-desc&page="

	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("could not start playwright: %v", err)
	}
	browser, err := pw.Chromium.Launch()
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	allLabels := make([]string, 0)

	for {
		fmt.Printf("Current page: %d\n", currentPage)
		currentUrl := fmt.Sprintf("%s%d", url, currentPage)
		if _, err = page.Goto(currentUrl); err != nil {
			log.Fatalf("could not goto: %v", err)
		}
		// get all the labels text .IssueLabel--big
		labels, err := page.Locator(".IssueLabel--big span").All()
		if err != nil {
			log.Fatalf("could not get labels: %v", err)
		}
		if len(labels) == 0 {
			break
		}
		for _, label := range labels {
			labelText, err := label.TextContent()
			if err != nil {
				log.Fatalf("could not get label text: %v", err)
				continue
			}
			allLabels = append(allLabels, labelText)
		}
		currentPage++
		fmt.Printf("Found %d labels so far\n", len(allLabels))
		fmt.Printf("Sleeping for 1 second\n")
		time.Sleep(time.Second * 1)
	}

	if err = browser.Close(); err != nil {
		log.Fatalf("could not close browser: %v", err)
	}
	if err = pw.Stop(); err != nil {
		log.Fatalf("could not stop Playwright: %v", err)
	}
	fmt.Printf("Found %d labels\n", len(allLabels))
	for _, label := range allLabels {
		fmt.Println(label)
	}
}

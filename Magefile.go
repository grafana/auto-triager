//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

type Run mg.Namespace
type Build mg.Namespace

// set default to build commands

var Default = Build.Commands

var archTargets = map[string]map[string]string{
	"darwin_amd64": {
		"CGO_ENABLED": "1",
		"GO111MODULE": "on",
		"GOARCH":      "amd64",
		"GOOS":        "darwin",
	},
	"darwin_arm64": {
		"CGO_ENABLED": "1",
		"GO111MODULE": "on",
		"GOARCH":      "arm64",
		"GOOS":        "darwin",
	},
	"linux_amd64": {
		"CGO_ENABLED": "1",
		"GO111MODULE": "on",
		"GOARCH":      "amd64",
		"GOOS":        "linux",
	},
}

func Clean() {
	log.Printf("Cleaning all")
	os.RemoveAll("./bin")
}

func buildCommand(command string, arch string) error {
	env, ok := archTargets[arch]
	if !ok {
		return fmt.Errorf("unknown arch %s", arch)
	}
	log.Printf("Building %s/%s\n", arch, command)
	outDir := fmt.Sprintf("./bin/%s/%s", arch, command)
	cmdDir := fmt.Sprintf("./pkg/cmd/%s", command)
	if err := sh.RunWith(env, "go", "build", "-o", outDir, cmdDir); err != nil {
		return err
	}

	// intentionally igores errors
	return sh.RunV("chmod", "+x", outDir)
}

func (Build) Commands(ctx context.Context) error {
	mg.Deps(
		Clean,
	)

	const commandsFolder = "./pkg/cmd"
	folders, err := os.ReadDir(commandsFolder)

	if err != nil {
		return err
	}

	for _, folder := range folders {
		if folder.IsDir() {
			currentArch := runtime.GOOS + "_" + runtime.GOARCH
			err := buildCommand(folder.Name(), currentArch)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (Run) Scrapper() error {
	mg.Deps(
		Build.Commands,
	)

	// check if a GH_TOKEN is defined in env if not fail
	if os.Getenv("GH_TOKEN") == "" {
		return fmt.Errorf("GH_TOKEN is not defined")
	}

	command := []string{
		"./bin/" + runtime.GOOS + "_" + runtime.GOARCH + "/scrapper",
	}

	return sh.RunV(command[0], command[1:]...)

}

func (Run) Triager(ctx context.Context, id string) error {
	mg.Deps(
		Build.Commands,
	)

	command := []string{
		"./bin/" + runtime.GOOS + "_" + runtime.GOARCH + "/triager",
		"-issueId=" + id,
		"-updateVectors=true",
		"-vectorDb=vector.db",
		"-issuesDb=github-data.sqlite",
	}

	return sh.RunV(command[0], command[1:]...)
}

func (Run) FineTuner(ctx context.Context, cmd string) error {
	mg.Deps(func() error {
		return buildCommand("fine-tuner", runtime.GOOS+"_"+runtime.GOARCH)
	})

	outFile := fmt.Sprintf("./out/fine-tune-dataset-%s.jsonl", cmd)

	command := []string{
		"./bin/" + runtime.GOOS + "_" + runtime.GOARCH + "/fine-tuner",
		"-issuesDb=github-data.sqlite",
		"-categorizedIdsFile=fixtures/categorizedIds.txt",
		"-missingInfoIdsFile=fixtures/missingInfoIds.txt",
		"-categorizableIdsFile=fixtures/categorizableIds.txt",
		fmt.Sprintf("-outFile=%s", outFile),
		cmd,
	}

	return sh.RunV(command[0], command[1:]...)
}

func (Run) TriagerFineTuned(ctx context.Context, id string) error {
	mg.Deps(func() error {
		return buildCommand("triager-fine-tuned", runtime.GOOS+"_"+runtime.GOARCH)
	})

	env := map[string]string{
		"DEBUG": "1",
	}

	command := []string{
		"./bin/" + runtime.GOOS + "_" + runtime.GOARCH + "/triager-fine-tuned",
		"-issueId=" + id,
	}

	return sh.RunWith(env, command[0], command[1:]...)
}

func (Run) ActionTester(ctx context.Context, id string) error {
	mg.Deps(func() error {
		return buildCommand("action-tester", runtime.GOOS+"_"+runtime.GOARCH)
	})

	command := []string{
		"./bin/" + runtime.GOOS + "_" + runtime.GOARCH + "/action-tester",
		"-issueId=" + id,
		"-repo=grafana/grafana-auto-triager-tests",
	}

	return sh.RunV(command[0], command[1:]...)
}

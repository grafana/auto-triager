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

func (Run) TriagerOpenAI(ctx context.Context, id string) error {
	mg.Deps(func() error {
		return buildCommand("triager-openai", runtime.GOOS+"_"+runtime.GOARCH)
	})

	env := map[string]string{
		"DEBUG": "1",
	}

	command := []string{
		"./bin/" + runtime.GOOS + "_" + runtime.GOARCH + "/triager-openai",
		// "-categorizerModel=ft:gpt-4o-2024-08-06:grafana-labs-experiments-exploration:auto-triage-categorizer:A1s2SnZR",
		"-issueId=" + id,
	}

	return sh.RunWith(env, command[0], command[1:]...)
}

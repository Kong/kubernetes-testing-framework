// Package main is a tool to trigger Github actions workflows
//
//
// Examples
//
// Trigger an integration test workflow for a custom branch:
//
//   $ go run internal/ci/release/main.go tests.yaml batman
//   SUCCESS: the test.yaml workflow has been triggered for batman
//
// Trigger a release testing workflow:
//
//   $ go run internal/ci/release/main.go release-testing.yaml <ref> <tag>
//   SUCCESS: the release-testing.yaml workflow has been triggered for <ref>
//
// Tag in this case is the tag that you want the release-testing.yaml workflow
// to create and push to the remote upon success (e.g. `v0.8.3`).
//
// Trigger a release workflow:
//
//   $ go run internal/ci/release/main.go release.yaml <ref>
//   SUCCESS: the release.yaml workflow has been triggered for <ref>
//
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"
)

var (
	githubOrg   = os.Getenv("GITHUB_ORG")
	githubRepo  = os.Getenv("GITHUB_REPO")
	githubToken = os.Getenv("GITHUB_TOKEN")
)

func main() {
	const (
		minArgs = 3
		maxArgs = 4
	)

	// ensure that the option, branch and tag arguments are provided.
	if len(os.Args) < minArgs || len(os.Args) > maxArgs {
		helpAndExit()
	}

	// gather the provided arguments
	workflow := os.Args[1]
	ref := os.Args[2]

	// validate that the workflow file exists
	if _, err := os.Stat(fmt.Sprintf(".github/workflows/%s", workflow)); err != nil {
		reportFailureAndExit(fmt.Errorf("ERROR: no workflow file %s was found in this repo: %w", workflow, err))
	}

	// validate that the required ENV vars were provided
	if githubOrg == "" {
		reportFailureAndExit(fmt.Errorf("ERROR: the GITHUB_ORG environment variable must be set"))
	}
	if githubRepo == "" {
		reportFailureAndExit(fmt.Errorf("ERROR: the GITHUB_REPO environment variable must be set"))
	}
	if githubToken == "" {
		reportFailureAndExit(fmt.Errorf("ERROR: the GITHUB_TOKEN environment variable must be set"))
	}

	// gather any optional tag that was provided
	var tag string
	if len(os.Args) == maxArgs {
		tag = os.Args[3]
	}

	// trigger the requested workflow
	if err := triggerGithubActionsWorkflow(ref, workflow, githubOrg, githubRepo, githubToken, tag); err != nil {
		reportFailureAndExit(err)
	}
}

// See https://docs.github.com/en/rest/reference/actions#create-a-workflow-triggerGithubActionsWorkflow-event
func triggerGithubActionsWorkflow(ref, workflow string, org, repo, token, tag string) error {
	// generate the Github API client using the OAuth Token provided
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	gh := github.NewClient(tc)

	// submit a dispatch event to trigger the workflow
	event := github.CreateWorkflowDispatchEventRequest{Ref: ref}
	if tag != "" { // if a tag was provided add that to the workflow inputs
		event.Inputs = map[string]interface{}{"tag": tag}
	}
	resp, err := gh.Actions.CreateWorkflowDispatchEventByFileName(ctx, org, repo, workflow, event)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// ensure the response from the Github API is as expected
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("expected 204 when submitting a dispatch event to github, got %s", resp.Status)
	}
	fmt.Printf("SUCCESS: the %s workflow has been triggered for %s\n", workflow, ref)

	return nil
}

// helpAndExit prints the usage information for the script and exits the program.
func helpAndExit() {
	fmt.Printf("usage: %s <workflow> <ref> <optional tag>\n", os.Args[0])
	os.Exit(1)
}

// reportFailureAndExit prints error information for non-recoverable failure conditions
// and then exits the program.
func reportFailureAndExit(err error) {
	fmt.Println(err.Error())
	os.Exit(1)
}

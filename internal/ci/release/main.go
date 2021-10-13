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
//   $ go run internal/ci/release/main.go release-testing.yaml <ref> tag=v0.8.3
//   SUCCESS: the release-testing.yaml workflow has been triggered for <ref>
//
// Where `tag=v0.8.3` are optional inputs that the `release-testing.yaml` uses
// specifically.
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
	"strings"

	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"
)

const (
	// githubTokenEnvVar is the environment variable that is used to pass in a Github OAuth Token
	githubTokenEnvVar = "GITHUB_TOKEN"

	// githubOrgEnvVar is the environment variable that is used to pass in the
	// Github Org which houses the relevant repository.
	githubOrgEnvVar = "GITHUB_ORG"

	// githubRepoEnvVar is the environment variable that is used to pass in the
	// Github Repo which workflows will be triggered on.
	githubRepoEnvVar = "GITHUB_REPO"
)

func main() {
	// ensure that the option, branch and tag arguments are provided.
	if len(os.Args) < 3 || len(os.Args) > 4 { //nolint:gomnd
		help()
	}

	// gather the provided arguments
	workflow := os.Args[1]
	ref := os.Args[2]

	// validate that the workflow file exists
	if _, err := os.Stat(fmt.Sprintf(".github/workflows/%s", workflow)); err != nil {
		fmt.Fprintf(os.Stderr, "no workflow file %s was found in this repo: %s\n", workflow, err)
		os.Exit(100)
	}

	// gather any optional workflow inputs if provided
	var inputsList string
	if len(os.Args) == 4 {
		inputsList = os.Args[3]
	}

	// build a map out of any provided inputs
	workflowInputs := make(map[string]string)
	for _, pair := range strings.Split(inputsList, ",") {
		kvs := strings.Split(pair, "=")
		for i := 1; i < len(kvs); i = i + 2 {
			workflowInputs[kvs[i-1]] = kvs[i]
		}
	}

	// trigger the requested workflow
	if err := triggerGithubActionsWorkflow(ref, workflow, workflowInputs); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(101)
	}
}

// help prints the usage information for the script and exits.
func help() {
	fmt.Printf("usage: %s <workflow> <ref> <optional k=v args>", os.Args[0])
	os.Exit(1)
}

// See https://docs.github.com/en/rest/reference/actions#create-a-workflow-triggerGithubActionsWorkflow-event
func triggerGithubActionsWorkflow(ref, workflow string, workflowInputs ...map[string]string) error {
	// verify that the caller provided a Github Org
	org := os.Getenv(githubOrgEnvVar)
	if org == "" {
		return fmt.Errorf("environment variable %s is required", githubOrgEnvVar)
	}

	// verify that the caller provided a Github Repo
	repo := os.Getenv(githubRepoEnvVar)
	if repo == "" {
		return fmt.Errorf("environment variable %s is required", githubRepoEnvVar)
	}

	// ensure the caller provided a Github API token
	token := os.Getenv(githubTokenEnvVar)
	if token == "" {
		return fmt.Errorf("%s can not be empty when running dispatch", githubTokenEnvVar)
	}

	// generate the Github API client using the OAuth Token provided
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	gh := github.NewClient(tc)

	// collect all workflow input data to send to the workflow
	workflowInputData := make(map[string]interface{})
	for _, workflowInput := range workflowInputs {
		for k, v := range workflowInput {
			workflowInputData[k] = v
		}
	}

	// submit a dispatch event to trigger the workflow
	event := github.CreateWorkflowDispatchEventRequest{Ref: ref, Inputs: workflowInputData}
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

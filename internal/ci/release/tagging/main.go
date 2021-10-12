// Package main includes tooling for CI to manage release tags.
//
// About
//
// This script is meant to be run at the end of a release CI workflow to
// perform the final release tagging after the rest of the release workflow has
// succeeded, and enables maintainers to use tags to trigger a release
// multiple times without having to manage or clean up tags.
//
// The specific problem this script was originally created to solve was to
// allow maintainers to avoid having to delete a tag if for some reason the
// release CI workflow was broken (e.g. the CI environment had a failure, or
// an unexpected test failure occurred, an image failed to push, e.t.c.).
//
// Usage
//
// The caller uses the GIT_REF environment variable with a specifically named
// tag (e.g. "do-release-v0.8.1", optionally "do-release-attempt-1-v0.8.1"):
//
//   $ GIT_REF=refs/tags/do-release-v0.8.1 go run internal/ci/release/tagging/main.go
//
// There is a "verify" option which only checks that the provided GIT_REF
// matches the expected pattern:
//
//   $ GIT_REF=refs/tags/do-release-v0.8.1 go run internal/ci/release/tagging/main.go verify
//   SUCCESS: v0.8.1 is a valid release tag
//
// This is meant to be used at the beginning of a release workflow to fail early
// before tests, builds, and image pushes run using a bad tag.
//
// Note that the tag should be in one of two formats:
//
//  - do-release-v<MAJOR>.<MINOR>.<PATCH>
//  - do-release-attempt-<ATTEMPT>-v<MAJOR>.<MINOR>.<PATCH>
//
// The "attempt-" option is just for convenience so a new release can be
// triggered without deleting the previous tag (this script will delete
// all the trigger tags automatically as part of its cleanup upon success).
//
// Another option is "retag":
//
//   $ GIT_REF=refs/tags/do-release-v0.8.1 go run internal/ci/release/tagging/main.go retag
//   SUCCESS: pushed tag v0.8.1 to remote
//
// This performs all the checks of verify above but also parses the semver
// version information from the provided GIT_REF to produce a tag. In the
// above example for instance this script will tag HEAD as `v0.8.1` and
// push that to the remote.
//
// Lastly the option "cleanup" can be provided, this will build a list of any
// remaining trigger tags and delete them.
//
// See Also:
//
//  - RELEASE.md
//  - .github/workflows/release-testing.yaml
//  - .github/workflow/release.yaml
//
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/blang/semver/v4"
)

// refEnvVar is the environment variable that is used to provide the git ref.
const refEnvVar = "GIT_REF"

// triggerRegex is the github ref regex that is used to verify that the tag is a valid
// release triggering tag and includes the semver version of the release tag
// that should ultimately be pushed upon CI success.
var triggerRegex = regexp.MustCompile("do-release-(attempt-[0-9]+-)?v([0-9]+).([0-9]+).([0-9]+)")

func main() {
	// ensure that only one option was provided
	if len(os.Args) != 2 { //nolint:gomnd
		help()
	}

	// gather the git ref from the environment
	ref := os.Getenv(refEnvVar)
	if ref == "" {
		fmt.Fprintf(os.Stderr, "environment variable %s was not set", refEnvVar)
		os.Exit(1)
	}

	// verify the ref and get the semver version
	version, err := verify(ref)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}

	// switch between verify and retag operational modes
	switch os.Args[1] {
	case "verify":
		fmt.Printf("SUCCESS: v%s is a valid release tag\n", version.String())
	case "retag":
		if err := retag(version); err != nil {
			fmt.Fprintf(os.Stderr, err.Error()+"\n")
			os.Exit(1)
		}
	case "cleanup":
		if err := cleanup(version); err != nil {
			fmt.Fprintf(os.Stderr, err.Error()+"\n")
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "%s is not a valid option\n", os.Args[1])
		help()
	}
}

// help prints the usage information for the script and exits.
func help() {
	fmt.Printf("usage: %s verify|retag\n", os.Args[0])
	os.Exit(1)
}

// verify validates the provided ref and checks that it matches our required
// pattern for trigger tags, and produces the semver.Version for that ref.
func verify(ref string) (*semver.Version, error) {
	// match the gitref with the pattern expected
	if !triggerRegex.Match([]byte(ref)) {
		return nil, fmt.Errorf("ref %s does not match, looking for prefix 'do-release-'", ref)
	}

	// verify that we received the correct number of matches
	matches := triggerRegex.FindAllStringSubmatch(ref, -1)
	if len(matches) != 1 {
		return nil, fmt.Errorf("ref %s should have had exactly one regex match, found %d", ref, len(matches))
	}

	// verify that we received the correct number of submatches
	submatches := matches[0]
	if len(submatches) != 5 { //nolint:gomnd
		return nil, fmt.Errorf("ref %s should have parsed into 5 parts, found %d", ref, len(submatches))
	}

	// pull the major, minor and patch versions from the sub matches
	majorVersion := submatches[2]
	minorVersion := submatches[3]
	patchVersion := submatches[4]

	// verify that the found version is valid semver
	version, err := semver.Parse(fmt.Sprintf("%s.%s.%s", majorVersion, minorVersion, patchVersion))
	if err != nil {
		return nil, fmt.Errorf("error while parsing version with semver: %s", err.Error())
	}

	return &version, err
}

// retag verifies the provided ref matches and extracts the final release tag
// that should be created for it, and pushes that tag to the remote upstream.
func retag(version *semver.Version) error {
	// generate the tag that will be used for this version
	tag := fmt.Sprintf("v%s", version.String())

	// create the tag in the local repo
	cmd := exec.Command("git", "tag", tag)
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to tag %s STDOUT=(%s) STDERR=(%s): %w", tag, stdout.String(), stderr.String(), err)
	}

	// push the tag to the remote
	cmd = exec.Command("git", "push", "--tags")
	stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push tags to remote STDOUT=(%s) STDERR=(%s): %w", stdout.String(), stderr.String(), err)
	}
	fmt.Printf("SUCCESS: pushed tag %s to remote\n", tag)

	return nil
}

// cleanup locates all trigger tags for the version being processed and deletes
// them at the upstream remote repository.
func cleanup(version *semver.Version) error {
	// find all the trigger tags that were created for this release
	triggerTags, err := findTriggerTags(version)
	if err != nil {
		return err
	}

	// delete each trigger tag on the remote
	for _, tag := range triggerTags {
		cmd := exec.Command("git", "push", "--delete", "origin", tag)
		stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			if !strings.Contains(err.Error(), "remote ref does not exist") { // tolerate tags that have already been deleted
				return fmt.Errorf("failed to delete tag %s STDOUT=(%s) STDERR=(%s): %w", tag, stdout.String(), stderr.String(), err)
			}
		}
	}

	// report on the cleanup effort
	success := new(bytes.Buffer)
	if len(triggerTags) > 0 {
		success.WriteString("SUCCESS: cleaned up trigger tags: ")
		for i, tag := range triggerTags {
			success.WriteString(tag)
			if i < (len(triggerTags) - 1) {
				success.WriteString(", ")
			}
		}
	} else {
		success.WriteString("SUCCESS: no cleanup required")
	}
	fmt.Println(success.String())

	return nil
}

// findTriggerTags fetches a list of all repository tags and identifies all the
// tags that match the version currently being processed.
func findTriggerTags(version *semver.Version) ([]string, error) {
	// fetch all tags from the remote
	cmd := exec.Command("git", "fetch", "--tags")
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not fetch tags STDOUT=(%s) STDERR=(%s): %w", stdout.String(), stderr.String(), err)
	}

	// pull a list of tags
	cmd = exec.Command("git", "tag")
	stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not list tags STDOUT=(%s) STDERR=(%s): %w", stdout.String(), stderr.String(), err)
	}

	// identify any tags that match the trigger tag regex
	triggerTagMap := make(map[string]*semver.Version)
	for _, tag := range strings.Split(stdout.String(), "\n") {
		if triggerRegex.Match([]byte(tag)) {
			tagVersion, err := verify(tag)
			if err != nil {
				return nil, err
			}
			triggerTagMap[tag] = tagVersion
		}
	}

	// filter for trigger tags that match the current version
	triggerTags := make([]string, 0)
	for tag, tagVersion := range triggerTagMap {
		if version.EQ(*tagVersion) {
			triggerTags = append(triggerTags, tag)
			delete(triggerTagMap, tag)
		}
	}

	// report warnings for any orphaned trigger tags for other versions
	for orphanedTag := range triggerTagMap {
		fmt.Fprintf(os.Stderr, "WARNING: orphaned tag %s was found. It doesn't belong to this version (%s) so it will need to be cleaned up manually.\n", orphanedTag, version.String())
	}

	return triggerTags, nil
}

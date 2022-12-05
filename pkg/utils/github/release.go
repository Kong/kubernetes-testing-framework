package github

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/blang/semver/v4"
	"github.com/google/go-github/v48/github"
	"golang.org/x/oauth2"
)

// FindLatestReleaseForRepo returns the latest release tag for a Github
// repository given an Organization and Repository name.
//
// NOTE: latest release in this context does not necessarily mean the newest
// version: if a repo releases a patch for an older release for instance
// the the returned version could be that instead.
func FindLatestReleaseForRepo(ctx context.Context, org, repo string) (*semver.Version, error) {
	rawTag, err := FindRawLatestReleaseForRepo(ctx, org, repo)
	if err != nil {
		return nil, err
	}

	latestVersion, err := semver.ParseTolerant(rawTag)
	if err != nil {
		return nil, fmt.Errorf("bad release tag returned from api when fetching latest %s/%s release tag: %w", org, repo, err)
	}

	return &latestVersion, err
}

// FindRawLatestReleaseForRepo returns the latest release tag as a string.
// It should be used directly for non-semver releases. Semver releases should use FindLatestReleaseForRepo
func FindRawLatestReleaseForRepo(ctx context.Context, org, repo string) (string, error) {
	var tc *http.Client
	if ghToken := os.Getenv("GITHUB_TOKEN"); ghToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: ghToken},
		)
		tc = oauth2.NewClient(ctx, ts)
	}
	client := github.NewClient(tc)

	release, _, err := client.Repositories.GetLatestRelease(context.Background(), org, repo)
	if err != nil {
		return "", fmt.Errorf("couldn't fetch latest %s/%s release: %w", org, repo, err)
	}

	releaseURL := release.URL
	if releaseURL == nil {
		return "", fmt.Errorf("release URL is nil for %s/%s", org, repo)
	}

	return *release.TagName, nil
}

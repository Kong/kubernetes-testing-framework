package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/blang/semver/v4"
)

const releaseURLFormatter = "https://api.github.com/repos/%s/%s/releases/latest"

// FindLatestReleaseForRepo returns the latest release tag for a Github
// repository given an Organization and Repository name.
//
// NOTE: latest release in this context does not necessarily mean the newest
//       version: if a repo releases a patch for an older release for instance
//       the the returned version could be that instead.
func FindLatestReleaseForRepo(org, repo string) (*semver.Version, error) {
	rawTag, err := FindRawLatestReleaseForRepo(org, repo)
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
func FindRawLatestReleaseForRepo(org, repo string) (string, error) {
	releaseURL := fmt.Sprintf(releaseURLFormatter, org, repo)
	resp, err := http.Get(releaseURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("couldn't determine latest %s/%s release: %w", org, repo, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	type latestReleaseData struct {
		TagName string `json:"tag_name"`
	}

	data := latestReleaseData{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("bad data from api when fetching latest %s/%s release tag: %w", org, repo, err)
	}

	return data.TagName, nil
}

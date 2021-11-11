package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

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
	releaseURL := fmt.Sprintf(releaseURLFormatter, org, repo)
	resp, err := http.Get(releaseURL) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("couldn't determine latest %s/%s release: %w", org, repo, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	type latestReleaseData struct {
		TagName string `json:"tag_name"`
	}

	data := latestReleaseData{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("bad data from api when fetching latest %s/%s release tag: %w", org, repo, err)
	}

	latestVersion, err := semver.Parse(strings.TrimPrefix(data.TagName, "v"))
	if err != nil {
		return nil, fmt.Errorf("bad release tag returned from api when fetching latest %s/%s release tag: %w", org, repo, err)
	}

	return &latestVersion, err
}

package test

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type checkerOptions struct {
	bodyChecker func(*testing.T, []byte) bool
}

type CheckerOption func(*checkerOptions)

func WithBodyChecker(bodyChecker func(*testing.T, []byte) bool) CheckerOption {
	return func(opts *checkerOptions) {
		opts.bodyChecker = bodyChecker
	}
}

// EventuallyExpectedStatusCodeAndBody is a helper function that retries the request until
// it gets the expected status. It also checks the body of the response if a body checker
// is provided. Default is to retry for 1 minute with 1 second interval.
func EventuallyExpectedStatusCodeAndBody(
	t *testing.T, httpClient *http.Client, req *http.Request, expectedStatusCode int, opts ...CheckerOption,
) {
	var options checkerOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	require.Eventually(
		t,
		func() bool {
			resp, err := httpClient.Do(req)
			if err != nil {
				t.Logf("WARNING: error while waiting for %s: %v", req.URL, err)
				return false
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Logf("WARNING: error cannot read response body %s: %v", req.URL, err)
				return false
			}
			if resp.StatusCode != expectedStatusCode {
				t.Logf("WARNING: unexpected response %s: %s with body: %s", req.URL, resp.Status, body)
				return false
			}
			if options.bodyChecker != nil && !options.bodyChecker(t, body) {
				t.Logf("WARNING: unexpected content of response body %s: %s", req.URL, body)
				return false
			}
			return true
		},
		time.Minute, time.Second,
	)
}

package test

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type httpResponseOption struct {
	bodyChecker       func(*testing.T, []byte) bool
	statusCodeChecker func(t *testing.T, reqURL string, statusCode int, body []byte) bool
}

type HTTPResponseOpt func(*httpResponseOption)

func WithBodyChecker(bodyChecker func(*testing.T, []byte) bool) HTTPResponseOpt {
	return func(opts *httpResponseOption) {
		opts.bodyChecker = bodyChecker
	}
}

func WithBodyContains(expected string) HTTPResponseOpt {
	return func(opts *httpResponseOption) {
		opts.bodyChecker = func(t *testing.T, body []byte) bool {
			t.Logf("check expected content %s of the response body", expected)
			return bytes.Contains(body, []byte(expected))
		}
	}
}

func WithStatusCode(expected int) HTTPResponseOpt {
	return func(opts *httpResponseOption) {
		opts.statusCodeChecker = func(t *testing.T, reqURL string, statusCode int, body []byte) bool {
			if statusCode != expected {
				t.Logf("WARNING: unexpected response %s: %d with body: %s", reqURL, statusCode, body)
				return false
			}
			t.Logf("expected status code %d of the response received", statusCode)
			return true
		}
	}
}

// EventuallyExpectedStatusCodeAndBody is a helper function that retries the request until
// it gets the expected response. For setting expected status code use WithStatusCode
// option (otherwise it's not checked). For checking content of body use WithBodyContains or
// WithBodyChecker (for fine-grained control), only one should be passed.
// Default is to retry for 1 minute with 1 second interval.
func EventuallyExpectedStatusCodeAndBody(
	t *testing.T, httpClient *http.Client, req *http.Request, opts ...HTTPResponseOpt,
) {
	var options httpResponseOption
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
			if options.statusCodeChecker != nil && !options.statusCodeChecker(t, req.URL.String(), resp.StatusCode, body) {
				return false
			} else {
				t.Log("skipping checking status code of the response")
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

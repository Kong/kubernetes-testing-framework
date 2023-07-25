package test

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	gokong "github.com/kong/go-kong/kong"
	"github.com/stretchr/testify/require"
)

type httpResponseExpectOpts struct {
	responseChecker []func(*testing.T, *http.Response) bool
	waitFor         time.Duration
	interval        time.Duration
}

type HTTPResponseExpectOpt func(*httpResponseExpectOpts)

func WithWaitFor(waitFor time.Duration) HTTPResponseExpectOpt {
	return func(opts *httpResponseExpectOpts) {
		opts.waitFor = waitFor
	}
}

func WithInterval(interval time.Duration) HTTPResponseExpectOpt {
	return func(opts *httpResponseExpectOpts) {
		opts.interval = interval
	}
}

func WithResponseChecker(responseChecker func(*testing.T, *http.Response) bool) HTTPResponseExpectOpt {
	return func(opts *httpResponseExpectOpts) {
		opts.responseChecker = append(opts.responseChecker, responseChecker) //nolint:bodyclose
	}
}

func WithBodyContains(s string) HTTPResponseExpectOpt {
	return WithResponseChecker(
		func(t *testing.T, resp *http.Response) bool {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Logf("WARNING: error cannot read response body returned by %s: %v", resp.Request.URL, err)
				return false
			}
			if !bytes.Contains(body, []byte(s)) {
				t.Logf("WARNING: unexpected content of response body returned by %s: %s", resp.Request.URL, body)
				return false
			}
			return true
		},
	)
}

func WithStatusCode(expected int) HTTPResponseExpectOpt {
	return WithResponseChecker(
		func(t *testing.T, resp *http.Response) bool {
			if resp.StatusCode != expected {
				t.Logf("WARNING: unexpected status code %d, expected %d", resp.StatusCode, expected)
				return false
			}
			return true
		},
	)
}

func WithEnterpriseHeader() HTTPResponseExpectOpt {
	return WithResponseChecker(
		func(t *testing.T, resp *http.Response) bool {
			version := strings.TrimPrefix(resp.Header.Get("Server"), "kong/")
			v, err := gokong.NewVersion(version)
			if err != nil {
				t.Logf("WARNING: error while parsing admin api version %s: %v", version, err)
				return false
			}
			t.Logf("admin api version %s", v)
			if !v.IsKongGatewayEnterprise() {
				t.Logf("WARNING: version %s should be an enterprise version but wasn't", v)
				return false
			}
			return true
		},
	)
}

// EventuallyExpectedResponse is a helper function that issues the provided request
// until it gets the expected response or the timeout (default: 1 minute) is reached.
// For assertions about the received response one can provide options like WithStatusCode or WithBodyContains.
// NOTE: only one option checking the body can be provided since body's reader can only
// be read once.
// Default is to retry for 1 minute with 1 second interval.
func EventuallyExpectResponse(
	t *testing.T, httpClient *http.Client, req *http.Request, opts ...HTTPResponseExpectOpt,
) {
	options := httpResponseExpectOpts{
		waitFor:  time.Minute,
		interval: time.Second,
	}
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
			if err != nil {
				t.Logf("WARNING: error cannot read response body %s: %v", req.URL, err)
				return false
			}
			for _, checker := range options.responseChecker {
				if !checker(t, resp) {
					return false
				}
			}
			return true
		},
		options.waitFor, options.interval,
	)
}

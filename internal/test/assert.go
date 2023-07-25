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

type configurationOption struct {
	responseChecker []func(*testing.T, *http.Response) bool
	waitFor         time.Duration
	interval        time.Duration
}

type ConfigurationOpt func(*configurationOption)

func WithWaitFor(waitFor time.Duration) ConfigurationOpt {
	return func(opts *configurationOption) {
		opts.waitFor = waitFor
	}
}

func WithInterval(interval time.Duration) ConfigurationOpt {
	return func(opts *configurationOption) {
		opts.interval = interval
	}
}

func WithResponseChecker(bodyChecker func(*testing.T, *http.Response) bool) ConfigurationOpt {
	return func(opts *configurationOption) {
		opts.responseChecker = append(opts.responseChecker, bodyChecker)
	}
}

func WithBodyContains(expected string) ConfigurationOpt {
	return WithResponseChecker(
		func(t *testing.T, resp *http.Response) bool {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Logf("WARNING: error cannot read response body %s: %v", resp.Request.URL, err)
				return false
			}
			if !bytes.Contains(body, []byte(expected)) {
				t.Logf("WARNING: unexpected content of response body %s: %s", resp.Request.URL, body)
				return false
			}
			t.Logf("expected content of the response body received")
			return true
		},
	)
}

func WithStatusCode(expected int) ConfigurationOpt {
	return WithResponseChecker(
		func(t *testing.T, resp *http.Response) bool {
			if resp.StatusCode != expected {
				t.Logf("WARNING: unexpected status code %d, expected %d", resp.StatusCode, expected)
				return false
			}
			t.Logf("expected status code received")
			return true
		},
	)
}

func WithEnterpriseHeader() ConfigurationOpt {
	return WithResponseChecker(
		func(t *testing.T, resp *http.Response) bool {
			version, _ := strings.CutPrefix(resp.Header.Get("Server"), "kong/")
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

// EventuallyExpectedResponse is a helper function that retries the request until
// it gets the expected response. For setting expected status code use WithStatusCode
// option (otherwise it's not checked). For checking content of body only one has to be
// passed (body can be read only one time).
// Default is to retry for 1 minute with 1 second interval.
func EventuallyExpectedResponse(
	t *testing.T, httpClient *http.Client, req *http.Request, opts ...ConfigurationOpt,
) {
	options := configurationOption{
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

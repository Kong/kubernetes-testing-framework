package networking

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// WaitForHTTP will make an HTTP GET request to the given URL until either
// the responses return the expected status code or the context completes (and
// then it will throw an error). The main purpose of this function is basically
// just to wait for an HTTP server that has been deployed to be ready without
// needing to bother with checking pod status, health e.t.c.
func WaitForHTTP(ctx context.Context, getURL string, statusCode int) chan error {
	errs := make(chan error)
	go func() {
		httpc := http.Client{Timeout: time.Second * 10} //nolint:mnd
		for {
			select {
			case <-ctx.Done():
				errs <- fmt.Errorf("context completed before path %s returned %d: %w", getURL, statusCode, ctx.Err())
				close(errs)
			default:
				resp, err := httpc.Get(getURL)
				if err != nil {
					continue
				}
				resp.Body.Close()
				if resp.StatusCode == statusCode {
					errs <- nil
					close(errs)
					return
				}
			}
		}
	}()
	return errs
}

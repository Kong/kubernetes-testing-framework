package docker

import (
	"context"

	"github.com/docker/docker/client"
)

// NewNegotiatedClientWithOpts is a wrapper around docker.NewClientWithOpts that negotiates the API version
// with the server.
func NewNegotiatedClientWithOpts(ctx context.Context, opts ...client.Opt) (*client.Client, error) {
	c, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	// Make sure the client API version matches server's API version to avoid discrepancy issues.
	c.NegotiateAPIVersion(ctx)
	return c, nil
}

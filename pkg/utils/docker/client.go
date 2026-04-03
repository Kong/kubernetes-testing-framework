package docker

import (
	"context"

	// "github.com/docker/docker/client"
	"github.com/moby/moby/client"
)

// NewNegotiatedClientWithOpts is a wrapper around docker.NewClientWithOpts that negotiates the API version
// with the server.
func NewNegotiatedClientWithOpts(ctx context.Context, opts ...client.Opt) (*client.Client, error) {
	c, err := client.New(opts...)
	if err != nil {
		return nil, err
	}
	_, err = c.Ping(ctx, client.PingOptions{})
	if err != nil {
		return nil, err
	}
	return c, nil
}

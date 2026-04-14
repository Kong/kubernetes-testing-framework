package docker

import (
	"context"

	"github.com/moby/moby/client"
)

// RunPrivilegedCommand is a very basic and opinionated helper function which runs the
// given command and arguments on the given container (by ID) privileged.
func RunPrivilegedCommand(ctx context.Context, containerID, command string, args ...string) error {
	// connect to the local docker env
	dockerc, err := NewNegotiatedClientWithOpts(ctx, client.FromEnv)
	if err != nil {
		return err
	}

	// load the exec command for the container
	execID, err := dockerc.ExecCreate(ctx, containerID, client.ExecCreateOptions{
		User:       "0",
		Privileged: true,
		Cmd:        append([]string{command}, args...),
	})
	if err != nil {
		return err
	}

	// run the command
	if _, err := dockerc.ExecStart(ctx, execID.ID, client.ExecStartOptions{}); err != nil {
		return err
	}

	return nil
}

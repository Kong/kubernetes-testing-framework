package retry

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/sirupsen/logrus"
)

const (
	retryCount = 10
	retryWait  = 3 * time.Second
)

type Doer interface {
	Do(ctx context.Context) error
	DoWithErrorHandling(ctx context.Context, errorFunc ErrorFunc) error
}

type ErrorFunc func(error, *bytes.Buffer, *bytes.Buffer) error

type commandDoer struct {
	cmd  string
	args []string
}

func Command(cmd string, args ...string) Doer {
	return commandDoer{
		cmd:  cmd,
		args: args,
	}
}

func (c commandDoer) Do(ctx context.Context) error {
	return retry.Do(func() error {
		cmd, stdout, stderr := c.createCmd(ctx)
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("command %q with args [%v] failed STDOUT=(%s) STDERR=(%s): %w",
				c.cmd, c.args, stdout.String(), stderr.String(), err,
			)
		}
		return nil
	},
		c.createOpts(ctx)...,
	)
}

// DoWithErrorHandling executes the command and runs errorFunc passing a resulting err, stdout and stderr to be handled
// by the caller. The errorFunc is going to be called always when the resulting err != nil.
func (c commandDoer) DoWithErrorHandling(ctx context.Context, errorFunc ErrorFunc) error {
	return retry.Do(func() error {
		cmd, stdout, stderr := c.createCmd(ctx)
		err := cmd.Run()
		if err != nil {
			return errorFunc(err, stdout, stderr)
		}
		return nil
	},
		c.createOpts(ctx)...,
	)
}

func (c commandDoer) createCmd(ctx context.Context) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, c.cmd, c.args...) //nolint:gosec
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd, stdout, stderr
}

func (c commandDoer) createOpts(ctx context.Context) []retry.Option {
	return []retry.Option{
		retry.Context(ctx),
		retry.Delay(retryWait),
		retry.Attempts(retryCount),
		retry.DelayType(retry.FixedDelay),
		retry.OnRetry(func(_ uint, err error) {
			if err != nil {
				logrus.WithError(err).
					WithField("args", c.args).
					Errorf("failed running %s", c.cmd)
			}
		}),
	}
}

package retry

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

type CommandDoer struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	cmd    string
	args   []string
}

func Command(cmd string, args ...string) CommandDoer {
	return CommandDoer{
		cmd:  cmd,
		args: args,
	}
}

func (c CommandDoer) WithStdin(r io.Reader) CommandDoer {
	c.stdin = r
	return c
}

func (c CommandDoer) WithStdout(w io.Writer) CommandDoer {
	c.stdout = w
	return c
}

func (c CommandDoer) WithStderr(w io.Writer) CommandDoer {
	c.stderr = w
	return c
}

func (c CommandDoer) Do(ctx context.Context) error {
	return retry.Do(func() error {
		cmd, stdout, stderr := c.createCmd(ctx)
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("command %q with args %v failed STDOUT=(%s) STDERR=(%s): %w",
				c.cmd, c.args, stdout.String(), stderr.String(), err,
			)
		}
		return nil
	},
		c.createOpts(ctx)...,
	)
}

// DoWithErrorHandling executes the command and runs errorFunc passing a resulting err, stdout and stderr to be handled
// by the caller. The errorFunc is going to be called only when the resulting err != nil.
func (c CommandDoer) DoWithErrorHandling(ctx context.Context, errorFunc ErrorFunc) error {
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

func (c CommandDoer) createCmd(ctx context.Context) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	stdout := new(bytes.Buffer)
	if c.stdout == nil {
		c.stdout = stdout
	} else {
		c.stdout = io.MultiWriter(c.stdout, stdout)
	}

	stderr := new(bytes.Buffer)
	if c.stderr == nil {
		c.stderr = stderr
	} else {
		c.stderr = io.MultiWriter(c.stderr, stderr)
	}

	cmd := exec.CommandContext(ctx, c.cmd, c.args...) //nolint:gosec
	cmd.Stdin = c.stdin
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr

	return cmd, stdout, stderr
}

func (c CommandDoer) createOpts(ctx context.Context) []retry.Option {
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

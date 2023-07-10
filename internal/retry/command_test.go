package retry_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kong/kubernetes-testing-framework/internal/retry"
)

func TestDoWithErrorHandling(t *testing.T) {
	t.Run("succeeded command won't call the errorFunc", func(t *testing.T) {
		cmd := retry.Command("echo", "test")

		itShouldntGetCalled := func(err error, _ *bytes.Buffer, _ *bytes.Buffer) error {
			t.Error("this function shouldn't be called because there was no error running command")
			return err
		}
		err := cmd.DoWithErrorHandling(context.Background(), itShouldntGetCalled)
		require.NoError(t, err)
	})

	t.Run("failing command will call the errorFunc", func(t *testing.T) {
		cmd := retry.Command("unknown-command")

		wasCalled := false
		itShouldBeCalled := func(err error, _ *bytes.Buffer, _ *bytes.Buffer) error {
			wasCalled = true
			return err
		}

		// Wait just a second to not make tests run too long. It's enough to know the errorFunc was called at least once.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := cmd.DoWithErrorHandling(ctx, itShouldBeCalled)
		require.Error(t, err)
		require.True(t, wasCalled, "expected errorFunc to be called because the command has failed")
	})
}

func TestDo(t *testing.T) {
	t.Run("passing stdout works", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		cmd := retry.Command("echo", "hello").WithStdout(stdout)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		err := cmd.Do(ctx)
		require.NoError(t, err)
		require.Equal(t, "hello\n", stdout.String())
	})

	t.Run("passing stdin works", func(t *testing.T) {
		stdin := bytes.NewBufferString("hello")
		cmd := retry.Command("cat").WithStdin(stdin)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		err := cmd.Do(ctx)
		require.NoError(t, err)
	})

	// Testing stderr might not be reliable because it's not guaranteed that
	// the command will fail in the time we allow it to run.
	// Alternative would be to wait long enough so that we're it ran but that
	// would unnecessarily make the tests longer.
}

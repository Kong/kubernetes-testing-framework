//go:build integration_tests
// +build integration_tests

package integration

import (
	"context"
)

// -----------------------------------------------------------------------------
// Testing Vars & Consts
// -----------------------------------------------------------------------------

var (
	// ctx is a common context that can be used between tests
	ctx = context.Background()
)

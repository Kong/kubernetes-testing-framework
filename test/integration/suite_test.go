//+build integration_tests

package integration

import (
	"context"
)

// -----------------------------------------------------------------------------
// Testing Vars & Consts
// -----------------------------------------------------------------------------

const (
	enterpriseLicenseEnvVar = "KONG_ENTERPRISE_LICENSE"
)

var (
	// ctx is a common context that can be used between tests
	ctx = context.Background()
)

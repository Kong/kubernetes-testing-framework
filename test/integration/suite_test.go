//go:build integration_tests
// +build integration_tests

package integration

import (
	"context"
	"os"
	"testing"
)

// -----------------------------------------------------------------------------
// Testing Vars & Consts
// -----------------------------------------------------------------------------

var (
	// ctx is a common context that can be used between tests
	ctx = context.Background()
)

// SkipEnterpriseTestIfNoEnv skips test cases that requires an enterprise license
// if environment variable KTF_TEST_RUN_ENTERPRISE_CASES is not set.
// The function should be called at the beginning of test cases requiring an enterprise license.
func SkipEnterpriseTestIfNoEnv(t *testing.T) {
	if os.Getenv("KTF_TEST_RUN_ENTERPRISE_CASES") != "true" {
		t.Skip("skipped the test requiring kong enterprise license")
	}
}

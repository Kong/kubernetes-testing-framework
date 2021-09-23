//+build integration_tests

package integration

import (
	"context"
	"time"
)

// -----------------------------------------------------------------------------
// Testing Vars & Consts
// -----------------------------------------------------------------------------

const (
	enterpriseLicenseEnvVar = "KONG_ENTERPRISE_LICENSE"

	httpBinImage = "kennethreitz/httpbin"
	httpbinWait  = time.Minute * 2

	ingressClassKey = "kubernetes.io/ingress.class"
	ingressClass    = "kong"

	waitTick = time.Second
)

var (
	// ctx is a common context that can be used between tests
	ctx = context.Background()
)

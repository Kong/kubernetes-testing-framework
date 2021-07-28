package ktf

import "time"

// -----------------------------------------------------------------------------
// Environments Vars
// -----------------------------------------------------------------------------

const (
	// DefaultEnvironmentName indicates the name that will be provided for
	// created environments if no other name is provided
	DefaultEnvironmentName = "kong-testing-environment"

	// EnvironmentCreateTimeout indicates the amount of time maximum that should
	// be allowed to wait for a test environment to finish creating.
	EnvironmentCreateTimeout = time.Minute * 5

	// EnvironmentDeleteTimeout indicates the amount of time maximum that should
	// be allowed to wait for a test environment to delete successfully.
	EnvironmentDeleteTimeout = time.Minute * 1
)

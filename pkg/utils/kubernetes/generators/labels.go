package generators

// -----------------------------------------------------------------------------
// Resource Labels
// -----------------------------------------------------------------------------

const (
	// TestResourceLabel is a label used on any resources to indicate that they
	// were created as part of a testing run and can be cleaned up in bulk based
	// on the value provided to the label.
	TestResourceLabel = "created-by-ktf"
)

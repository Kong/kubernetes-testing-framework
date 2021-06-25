package kong

// -----------------------------------------------------------------------------
// DBMode
// -----------------------------------------------------------------------------

// DBMode indicate which storage backend the Kong Proxy should be deployed with (e.g. DBLESS, Postgres, e.t.c.)
type DBMode string

const (
	// DBLESS indicates that the Kong Proxy should be deployed with the DBLESS storage backend.
	DBLESS DBMode = "dbless"

	// PostGreSQL indicates that the Kong Proxy should be deployed with a PostGreSQL storage backend.
	PostGreSQL DBMode = "postgres"
)

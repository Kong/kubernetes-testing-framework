package gke

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeCreatedByID(t *testing.T) {
	sanitized := sanitizeCreatedByID("764086051850-6qr4p6gpi6hn506pt8ejuq83di345HUR.apps^googleusercontent$com")
	require.Equal(t, "764086051850-6qr4p6gpi6hn506pt8ejuq83di345hur-apps-googleuserco", sanitized,
		"expected disallowed characters to be replaced with dashes, capitals to be lowered, and output to be truncated")
}

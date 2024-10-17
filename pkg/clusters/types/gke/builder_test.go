package gke

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeCreatedByID(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "longer than allowed",
			input:    "764086051850-6qr4p6gpi6hn506pt8ejuq83di345HUR.apps^googleusercontent$com",
			expected: "764086051850-6qr4p6gpi6hn506pt8ejuq83di345hur-apps-googleuserco",
		},
		{
			name:     "short",
			input:    "764086051850",
			expected: "764086051850",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sanitized := sanitizeCreatedByID(tc.input)
			require.Equal(t, tc.expected, sanitized)
		})
	}
}

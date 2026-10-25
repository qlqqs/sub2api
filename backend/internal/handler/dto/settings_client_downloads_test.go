//go:build unit

package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseClientDownloads(t *testing.T) {
	for _, raw := range []string{"", " ", "[]", "null", "{}", "broken", `[{}, {"links":"invalid"}]`} {
		t.Run(raw, func(t *testing.T) {
			items := ParseClientDownloads(raw)
			require.NotNil(t, items)
			require.Empty(t, items)
			encoded, err := json.Marshal(items)
			require.NoError(t, err)
			require.Equal(t, "[]", string(encoded))
		})
	}
	items := ParseClientDownloads(`[{"id":"example","name":"Client","links":[{"label":"Linux","platform":"linux","arch":"x64","url":"https://example.com/client"}]}]`)
	require.Len(t, items, 1)
	require.Equal(t, "example", items[0].ID)
	require.Len(t, items[0].Links, 1)
	require.Equal(t, "linux", items[0].Links[0].Platform)
}

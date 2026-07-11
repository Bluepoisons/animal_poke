package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNarrative_FailForwardReasons(t *testing.T) {
	r, _ := setupNarrative(t)
	tok := narrAuth(t, r, "ff-device-01")
	for _, tc := range []struct {
		reason string
		want   string
	}{
		{"weather", "ff_weather"},
		{"no_camera", "ff_no_camera"},
		{"permission", "ff_no_camera"},
		{"miss", "ff_miss_1"},
	} {
		w := narrJSON(t, r, "POST", "/api/v1/narrative/fail-forward", tok, map[string]any{
			"miss_count": 1, "reason": tc.reason,
		})
		require.Equal(t, 200, w.Code, w.Body.String())
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		node := resp["node"].(map[string]any)
		assert.Equal(t, tc.want, node["node_id"], tc.reason)
	}
	// three misses
	w := narrJSON(t, r, "POST", "/api/v1/narrative/fail-forward", tok, map[string]any{
		"miss_count": 3, "reason": "miss",
	})
	require.Equal(t, 200, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ff_miss_3", resp["node"].(map[string]any)["node_id"])
}

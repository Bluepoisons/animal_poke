package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRedisSharedCounter_RejectsInsecureURLs(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{name: "rediss_no_password", url: "rediss://redis.example:6379/0"},
		{name: "rediss_skip_verify", url: "rediss://:secret@redis.example:6379/0?skip_verify=true"},
		{name: "rediss_empty_host", url: "rediss://:secret@/0"},
		{name: "bad_scheme", url: "http://redis.example:6379/0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRedisSharedCounter(tc.url)
			require.Error(t, err)
		})
	}
}

func TestNewRedisSharedCounter_DevPlaintextParses(t *testing.T) {
	// Will fail on ping unless Redis is up; we only assert it does not fail policy validation first.
	// Use an unroutable port and accept ping error that is NOT a policy error.
	_, err := NewRedisSharedCounter("redis://127.0.0.1:1/0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis ping")
}

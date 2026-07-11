package contentmanifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild_DefaultPackagesAndETag(t *testing.T) {
	s := NewStore("test-signing-key")
	m, err := s.Build(Filter{Locale: "zh", Region: "CN", ClientVersion: "1.2.0"})
	require.NoError(t, err)
	assert.Equal(t, SchemaVersion, m.SchemaVersion)
	assert.Equal(t, "zh-CN", m.Locale)
	assert.NotEmpty(t, m.ContentVersion)
	assert.NotEmpty(t, m.ETag)
	assert.NotEmpty(t, m.Signature)
	assert.False(t, m.Revoked)
	require.NotEmpty(t, m.Packages)
	kinds := map[string]bool{}
	for _, p := range m.Packages {
		kinds[p.Kind] = true
	}
	assert.True(t, kinds["species"])
	assert.True(t, kinds["quest"])
	assert.True(t, kinds["chapter"])
	assert.True(t, VerifySignature([]byte("test-signing-key"), m))
}

func TestBuild_OldClientRejected(t *testing.T) {
	s := NewStore("")
	_, err := s.Build(Filter{ClientVersion: "0.0.1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "below min")
}

func TestPublishRevokeRollback(t *testing.T) {
	s := NewStore("k")
	base, err := s.Build(Filter{Locale: "en"})
	require.NoError(t, err)

	pkgs := []PackageRef{{Kind: "species", ID: "species.cat", Version: "v9", Locale: "en", URL: "/content/species/cat"}}
	require.NoError(t, s.Publish("content-test-2", pkgs))
	m2, err := s.Build(Filter{Locale: "en"})
	require.NoError(t, err)
	assert.Equal(t, "content-test-2", m2.ContentVersion)
	assert.Len(t, m2.Packages, 1)

	s.Revoke()
	m3, err := s.Build(Filter{Locale: "en"})
	require.NoError(t, err)
	assert.True(t, m3.Revoked)

	require.NoError(t, s.Rollback())
	m4, err := s.Build(Filter{Locale: "en"})
	require.NoError(t, err)
	assert.Equal(t, base.ContentVersion, m4.ContentVersion)
	assert.False(t, m4.Revoked)
}

func TestPublish_RejectsUnsafe(t *testing.T) {
	s := NewStore("")
	err := s.Publish("x", []PackageRef{{
		Kind: "species", ID: "species.cat", Version: "v1", URL: "javascript:alert(1)",
	}})
	require.Error(t, err)
}

func TestETagStableAcrossGeneratedAt(t *testing.T) {
	s := NewStore("")
	m1, err := s.Build(Filter{Locale: "zh-CN"})
	require.NoError(t, err)
	m2, err := s.Build(Filter{Locale: "zh-CN"})
	require.NoError(t, err)
	assert.Equal(t, m1.ETag, m2.ETag)
}

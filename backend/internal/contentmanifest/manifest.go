// Package contentmanifest implements AP-080 versioned multi-locale content delivery.
package contentmanifest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"animalpoke/backend/internal/narrativecatalog"
	"animalpoke/backend/internal/questcatalog"
	"animalpoke/backend/internal/speciespack"
)

const (
	// SchemaVersion is the manifest envelope schema; clients fallback when incompatible.
	SchemaVersion = "content-manifest.v1"
	// MinClientVersion is the oldest client that understands this schema.
	MinClientVersion = "1.0.0"
)

// PackageRef is a content package pointer (no large payloads inline).
type PackageRef struct {
	Kind     string            `json:"kind"`
	ID       string            `json:"id"`
	Version  string            `json:"version"`
	Locale   string            `json:"locale,omitempty"`
	URL      string            `json:"url,omitempty"`
	Checksum string            `json:"checksum,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// Manifest is the filtered content delivery payload.
type Manifest struct {
	SchemaVersion    string       `json:"schema_version"`
	ContentVersion   string       `json:"content_version"`
	Locale           string       `json:"locale"`
	Region           string       `json:"region,omitempty"`
	MinClientVersion string       `json:"min_client_version"`
	GeneratedAt      string       `json:"generated_at"`
	Revoked          bool         `json:"revoked"`
	Packages         []PackageRef `json:"packages"`
	ETag             string       `json:"etag"`
	Signature        string       `json:"signature,omitempty"`
}

// Filter selects which content a client may receive.
type Filter struct {
	Locale            string
	Region            string
	AgeRating         string // e.g. everyone | teen
	ClientVersion     string
	MinClientVersionQ string // client-declared min it can handle (optional)
}

// Store holds current/previous/revoked content versions (process-local, deterministic).
type Store struct {
	mu       sync.RWMutex
	current  release
	previous *release
	signing  []byte
}

type release struct {
	Version   string
	Revoked   bool
	CreatedAt time.Time
	// Overrides replaces default built packages when non-nil (tests / ops publish).
	Overrides []PackageRef
}

// NewStore creates a content store. signingKey empty → signature omitted.
func NewStore(signingKey string) *Store {
	s := &Store{
		current: release{
			Version:   defaultContentVersion(),
			CreatedAt: time.Now().UTC(),
		},
		signing: []byte(strings.TrimSpace(signingKey)),
	}
	return s
}

func defaultContentVersion() string {
	return fmt.Sprintf("content-%s", time.Now().UTC().Format("20060102"))
}

// Publish replaces current release (keeps previous for rollback).
func (s *Store) Publish(version string, packages []PackageRef) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return errors.New("version required")
	}
	if err := validatePackages(packages); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.current
	s.previous = &prev
	s.current = release{
		Version:   version,
		CreatedAt: time.Now().UTC(),
		Overrides: append([]PackageRef(nil), packages...),
	}
	return nil
}

// Revoke marks the current release revoked (clients must not use it as LKG).
func (s *Store) Revoke() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current.Revoked = true
}

// Rollback restores previous release when available.
func (s *Store) Rollback() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.previous == nil {
		return errors.New("no previous content version")
	}
	s.current = *s.previous
	s.previous = nil
	return nil
}

// CurrentVersion returns the active content version string.
func (s *Store) CurrentVersion() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current.Version
}

// Build constructs a filtered manifest for the client.
func (s *Store) Build(f Filter) (Manifest, error) {
	s.mu.RLock()
	rel := s.current
	key := append([]byte(nil), s.signing...)
	s.mu.RUnlock()

	locale := normalizeLocale(f.Locale)
	if locale == "" {
		locale = "zh-CN"
	}
	region := strings.TrimSpace(f.Region)
	if region == "" {
		region = "CN"
	}

	if !clientCompatible(f.ClientVersion, MinClientVersion) {
		return Manifest{}, fmt.Errorf("client_version %q below min %s", f.ClientVersion, MinClientVersion)
	}

	packages := rel.Overrides
	if packages == nil {
		packages = buildDefaultPackages(locale, region, f.AgeRating)
	}
	packages = filterPackages(packages, locale, region, f.AgeRating)

	m := Manifest{
		SchemaVersion:    SchemaVersion,
		ContentVersion:   rel.Version,
		Locale:           locale,
		Region:           region,
		MinClientVersion: MinClientVersion,
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		Revoked:          rel.Revoked,
		Packages:         packages,
	}
	etag, err := computeETag(m)
	if err != nil {
		return Manifest{}, err
	}
	m.ETag = etag
	if len(key) > 0 {
		m.Signature = signManifest(key, m)
	}
	return m, nil
}

func validatePackages(pkgs []PackageRef) error {
	for _, p := range pkgs {
		if strings.TrimSpace(p.Kind) == "" || strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.Version) == "" {
			return errors.New("package kind/id/version required")
		}
		// Reject obvious script injection in free-text labels/urls.
		if containsUnsafeMarkdown(p.URL) || containsUnsafeMarkdown(p.ID) {
			return errors.New("package contains unsafe content")
		}
	}
	return nil
}

func containsUnsafeMarkdown(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, "<script") || strings.Contains(low, "javascript:")
}

func normalizeLocale(locale string) string {
	l := strings.TrimSpace(locale)
	if l == "" {
		return ""
	}
	l = strings.ReplaceAll(l, "_", "-")
	parts := strings.Split(l, "-")
	if len(parts) == 1 {
		switch strings.ToLower(parts[0]) {
		case "zh":
			return "zh-CN"
		case "en":
			return "en"
		case "ja":
			return "ja"
		default:
			return l
		}
	}
	return parts[0] + "-" + strings.ToUpper(parts[1])
}

func buildDefaultPackages(locale, region, age string) []PackageRef {
	_ = age
	packs := speciespack.Default().All()
	out := make([]PackageRef, 0, len(packs)+8)
	for _, p := range packs {
		out = append(out, PackageRef{
			Kind:    "species",
			ID:      p.ContentID,
			Version: p.Version,
			Locale:  locale,
			URL:     fmt.Sprintf("/content/species/%s", p.ID),
			Labels: map[string]string{
				"status": p.Status,
				"region": region,
			},
		})
	}
	// Quest catalog reference
	out = append(out, PackageRef{
		Kind:    "quest",
		ID:      "quest-catalog",
		Version: questcatalog.ConfigVersion,
		Locale:  locale,
		URL:     "/api/v1/quests/catalog",
	})
	// Narrative chapters
	out = append(out, PackageRef{
		Kind:    "chapter",
		ID:      "narrative",
		Version: narrativecatalog.ContentVersion,
		Locale:  locale,
		URL:     "/api/v1/narrative/catalog",
	})
	// Live events placeholder package (definition only; instance state is AP-081)
	out = append(out, PackageRef{
		Kind:    "event",
		ID:      "live-events",
		Version: "events.v0",
		Locale:  locale,
		URL:     "/content/events/index.json",
		Labels:  map[string]string{"status": "placeholder"},
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].ID < out[j].ID
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func filterPackages(pkgs []PackageRef, locale, region, age string) []PackageRef {
	_ = region
	out := make([]PackageRef, 0, len(pkgs))
	for _, p := range pkgs {
		if p.Locale != "" && !localeMatches(p.Locale, locale) {
			// Prefer exact locale packages; keep unscoped packages.
			continue
		}
		if age == "everyone" && p.Labels != nil && p.Labels["age"] == "teen" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func localeMatches(pkgLocale, want string) bool {
	if strings.EqualFold(pkgLocale, want) {
		return true
	}
	// zh-CN accepts zh
	return strings.HasPrefix(strings.ToLower(want), strings.ToLower(pkgLocale))
}

// Semver-ish major.minor compare; empty client version is allowed (dev).
func clientCompatible(client, min string) bool {
	if strings.TrimSpace(client) == "" {
		return true
	}
	return compareSemver(client, min) >= 0
}

func compareSemver(a, b string) int {
	ap := parseSemver(a)
	bp := parseSemver(b)
	for i := 0; i < 3; i++ {
		if ap[i] < bp[i] {
			return -1
		}
		if ap[i] > bp[i] {
			return 1
		}
	}
	return 0
}

func parseSemver(s string) [3]int {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	var out [3]int
	parts := strings.Split(s, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n := 0
		for _, ch := range parts[i] {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		out[i] = n
	}
	return out
}

func computeETag(m Manifest) (string, error) {
	// ETag over stable fields excluding signature and generated_at.
	stable := struct {
		Schema         string       `json:"schema_version"`
		ContentVersion string       `json:"content_version"`
		Locale         string       `json:"locale"`
		Region         string       `json:"region"`
		MinClient      string       `json:"min_client_version"`
		Revoked        bool         `json:"revoked"`
		Packages       []PackageRef `json:"packages"`
	}{
		Schema: m.SchemaVersion, ContentVersion: m.ContentVersion, Locale: m.Locale,
		Region: m.Region, MinClient: m.MinClientVersion, Revoked: m.Revoked, Packages: m.Packages,
	}
	raw, err := json.Marshal(stable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return `"` + hex.EncodeToString(sum[:16]) + `"`, nil
}

func signManifest(key []byte, m Manifest) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(m.ETag))
	_, _ = mac.Write([]byte(m.ContentVersion))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature checks HMAC over etag+version.
func VerifySignature(key []byte, m Manifest) bool {
	if len(key) == 0 || m.Signature == "" {
		return false
	}
	return hmac.Equal([]byte(m.Signature), []byte(signManifest(key, m)))
}

var defaultStore *Store
var defaultOnce sync.Once

// Default returns process-wide store (signing via CONTENT_MANIFEST_HMAC if set later).
func Default() *Store {
	defaultOnce.Do(func() {
		defaultStore = NewStore("")
	})
	return defaultStore
}

// SetDefaultSigningKey configures HMAC for the default store (tests/main).
func SetDefaultSigningKey(key string) {
	s := Default()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signing = []byte(strings.TrimSpace(key))
}

package config

import (
	"strings"
	"time"
)

// DeprecatedOperation records a sunset schedule for one API operation.
type DeprecatedOperation struct {
	Method     string // GET / POST / PUT / DELETE
	Path       string // e.g. "/api/v1/animals"
	SunsetDate time.Time
	Migration  string // doc fragment or replacement path
}

// VersionConfig drives the version negotiation middleware.
type VersionConfig struct {
	MinClientVersion     string
	DeprecatedOperations map[string]DeprecatedOperation // key = "METHOD /api/v1/path"
}

// IsDeprecated checks whether the given operation is on the deprecation schedule.
func (vc VersionConfig) IsDeprecated(method, path string) (DeprecatedOperation, bool) {
	if vc.DeprecatedOperations == nil {
		return DeprecatedOperation{}, false
	}
	key := strings.ToUpper(method) + " " + path
	op, ok := vc.DeprecatedOperations[key]
	return op, ok
}

// DefaultAPIVersion is the current API version prefix.
const DefaultAPIVersion = "v1"

// MinClientVersion returns the minimum supported client version, or empty
// string when no minimum is enforced.
func (c *Config) MinClientVersion() string { return c.minClientVersion }

// VersionConfig builds the middleware config from server configuration.
func (c *Config) VersionConfig() VersionConfig {
	return VersionConfig{
		MinClientVersion:     c.minClientVersion,
		DeprecatedOperations: c.DeprecatedOperations(),
	}
}

// DeprecatedOperations returns the current deprecation registry.
func (c *Config) DeprecatedOperations() map[string]DeprecatedOperation {
	if c.deprecatedOps != nil {
		return c.deprecatedOps
	}
	return map[string]DeprecatedOperation{}
}

// RegisterDeprecation adds an operation to the deprecation schedule.
func (c *Config) RegisterDeprecation(method, path, migration string, sunset time.Time) {
	if c.deprecatedOps == nil {
		c.deprecatedOps = make(map[string]DeprecatedOperation)
	}
	key := strings.ToUpper(method) + " " + path
	c.deprecatedOps[key] = DeprecatedOperation{
		Method:     method,
		Path:       path,
		SunsetDate: sunset,
		Migration:  migration,
	}
}

// Package middleware — API version negotiation and deprecation signalling.
package middleware

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"animalpoke/backend/internal/config"

	"github.com/gin-gonic/gin"
)

// Version negotiates client version and adds deprecation/sunset headers.
func Version(cfg config.VersionConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientVersion := c.GetHeader("X-Client-Version")
		if clientVersion == "" {
			clientVersion = "unknown"
		}

		c.Set("client_version", clientVersion)

		// Minimum client version gate
		if cfg.MinClientVersion != "" && clientVersion != "unknown" {
			if versionLess(clientVersion, cfg.MinClientVersion) {
				AbortJSON(c, http.StatusUpgradeRequired, "client_too_old",
					"minimum client version is "+cfg.MinClientVersion, false,
					map[string]any{
						"min_client_version":         cfg.MinClientVersion,
						"current_client_version":     clientVersion,
						"recommended_update_channel": "app_store",
					})
				return
			}
		}

		// Capability echo
		clientCaps := c.GetHeader("X-Client-Capabilities")
		if clientCaps != "" {
			c.Set("client_capabilities", parseCapabilities(clientCaps))
		}

		c.Next()

		// Add deprecation/sunset headers if the matched route is deprecated
		if op, ok := cfg.IsDeprecated(c.Request.Method, c.FullPath()); ok {
			c.Header("Deprecation", "true")
			c.Header("Sunset", op.SunsetDate.UTC().Format(time.RFC1123))
			if op.Migration != "" {
				c.Header("Warning", `299 - "`+op.Migration+`"`)
			}
		}

		// Always advertise minimum version
		if cfg.MinClientVersion != "" {
			c.Header("X-Min-Client-Version", cfg.MinClientVersion)
		}
		c.Header("X-Client-Version-Received", clientVersion)
	}
}

func versionLess(a, b string) bool {
	partsA := splitVersion(a)
	partsB := splitVersion(b)
	for i := 0; i < 3; i++ {
		if partsA[i] < partsB[i] {
			return true
		}
		if partsA[i] > partsB[i] {
			return false
		}
	}
	return false
}

func splitVersion(v string) [3]int {
	var parts [3]int
	for i, s := range strings.SplitN(v, ".", 3) {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			n = 0
		}
		parts[i] = n
	}
	return parts
}

func parseCapabilities(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

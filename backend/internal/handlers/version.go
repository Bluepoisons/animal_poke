package handlers

import (
	"net/http"

	"animalpoke/backend/internal/config"

	"github.com/gin-gonic/gin"
)

// VersionInfo is the response shape for the version negotiation endpoint.
type VersionInfo struct {
	APIVersion         string   `json:"api_version"`
	MinClientVersion   string   `json:"min_client_version,omitempty"`
	ServerCapabilities []string `json:"server_capabilities,omitempty"`
}

// Version returns the current API version and minimum supported client version.
func Version(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		info := VersionInfo{
			APIVersion:       config.DefaultAPIVersion,
			MinClientVersion: cfg.MinClientVersion(),
		}

		// Echo supported server capabilities
		if cfg.FeatureFlags.Ranking {
			info.ServerCapabilities = append(info.ServerCapabilities, "ranking")
		}
		if cfg.FeatureFlags.PvP {
			info.ServerCapabilities = append(info.ServerCapabilities, "pvp")
		}
		if cfg.FeatureFlags.Social {
			info.ServerCapabilities = append(info.ServerCapabilities, "social")
		}

		c.JSON(http.StatusOK, info)
	}
}

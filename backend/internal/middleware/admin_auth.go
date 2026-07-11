// Package middleware — 管理端 RBAC 鉴权（AP-085）。
// 主路径：短期 Admin JWT；break-glass：可选 X-Admin-Key + Actor + Reason。
// 生产禁止共享 key 作为常规认证。
package middleware

import (
	"log/slog"
	"strings"
	"time"

	"animalpoke/backend/internal/admin"

	"github.com/gin-gonic/gin"
)

const (
	// ContextKeyAdminActor 管理端 Actor 对象。
	ContextKeyAdminActor = "admin_actor"
	// HeaderAdminKey 共享 break-glass 密钥头。
	HeaderAdminKey = "X-Admin-Key"
	// HeaderAdminActor break-glass 真实操作人。
	HeaderAdminActor = "X-Admin-Actor"
	// HeaderAdminReason 操作原因（审计必填，写操作）。
	HeaderAdminReason = "X-Admin-Reason"
)

// AdminAuthConfig 管理端鉴权配置。
type AdminAuthConfig struct {
	// Tokens 签发/校验服务（可 nil：仅允许 break-glass）。
	Tokens *admin.TokenService
	// Sessions 会话存储（可 nil：跳过撤权检查）。
	Sessions *admin.SessionStore
	// Auditor 动作审计（可 nil：跳过写审计）。
	Auditor *admin.ActionAuditor
	// AdminAPIKey 共享密钥（仅 break-glass）。
	AdminAPIKey string
	// BreakGlassEnabled 是否允许 X-Admin-Key 紧急入口。
	BreakGlassEnabled bool
	// Production 生产环境：禁止共享 key 常规认证。
	Production bool
	// Env 环境名（写入 actor）。
	Env string
	// RequireReasonOnWrite 写方法是否强制 X-Admin-Reason。
	RequireReasonOnWrite bool
}

// AdminAuthRBAC 管理端鉴权中间件：Admin JWT 优先，可选 break-glass。
func AdminAuthRBAC(cfg AdminAuthConfig) gin.HandlerFunc {
	if cfg.Env == "" {
		cfg.Env = "development"
	}
	return func(c *gin.Context) {
		reason := strings.TrimSpace(c.GetHeader(HeaderAdminReason))
		if cfg.RequireReasonOnWrite && isWriteMethod(c.Request.Method) && reason == "" {
			AbortBadRequest(c, "admin_reason_required", "X-Admin-Reason required for admin writes", nil)
			return
		}

		// 1) Bearer Admin JWT（主路径）
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && cfg.Tokens != nil {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				actor, err := cfg.Tokens.Parse(parts[1])
				if err != nil {
					slog.Warn("admin jwt invalid", "err", err)
					AbortUnauthorized(c, "invalid_admin_token", "invalid admin token")
					return
				}
				if cfg.Sessions != nil && actor.SessionID != "" {
					ok, err := cfg.Sessions.IsActive(actor.SessionID, time.Now().UTC())
					if err != nil || !ok {
						AbortUnauthorized(c, "admin_session_revoked", "admin session revoked or expired")
						return
					}
				}
				if reason != "" {
					c.Set("admin_reason", reason)
				}
				setAdminActor(c, actor)
				c.Next()
				return
			}
		}

		// 2) Break-glass：X-Admin-Key + Actor + Reason（生产需显式开启）
		key := strings.TrimSpace(c.GetHeader(HeaderAdminKey))
		if key != "" {
			if cfg.AdminAPIKey == "" {
				AbortForbidden(c, "admin_not_configured", "admin not configured")
				return
			}
			if key != cfg.AdminAPIKey {
				AbortForbidden(c, "forbidden", "forbidden")
				return
			}
			// 生产且未开 break-glass：禁止共享 key 常规认证
			if cfg.Production && !cfg.BreakGlassEnabled {
				AbortForbidden(c, "shared_key_forbidden", "shared admin key is not allowed as regular production auth; use admin JWT or enable break-glass")
				return
			}
			if !cfg.BreakGlassEnabled {
				AbortForbidden(c, "break_glass_disabled", "break-glass admin key auth is disabled")
				return
			}
			actorID := strings.TrimSpace(c.GetHeader(HeaderAdminActor))
			if actorID == "" {
				AbortBadRequest(c, "admin_actor_required", "X-Admin-Actor required for break-glass", nil)
				return
			}
			if reason == "" {
				AbortBadRequest(c, "admin_reason_required", "X-Admin-Reason required for break-glass", nil)
				return
			}
			actor := &admin.Actor{
				Subject:   "break-glass:" + actorID,
				ActorID:   actorID,
				Role:      admin.RoleSuper,
				SessionID: "break-glass",
				AuthMode:  "break_glass",
				Env:       cfg.Env,
			}
			c.Set("admin_reason", reason)
			setAdminActor(c, actor)
			c.Next()
			return
		}

		if cfg.Tokens == nil && cfg.AdminAPIKey == "" {
			AbortForbidden(c, "admin_not_configured", "admin not configured")
			return
		}
		AbortUnauthorized(c, "admin_auth_required", "admin JWT or break-glass credentials required")
	}
}

// AdminAuth 兼容旧签名：映射为 break-glass 开启的 RBAC 中间件（测试/开发）。
// 生产路由必须使用显式 AdminAuthRBAC。
func AdminAuth(adminKey string) gin.HandlerFunc {
	return AdminAuthRBAC(AdminAuthConfig{
		AdminAPIKey:       adminKey,
		BreakGlassEnabled: true,
		Production:        false,
		Env:               "test",
	})
}

// AdminAuthLegacyKeyOnly 仅测试：接受单独 X-Admin-Key 并注入固定 actor=admin。
func AdminAuthLegacyKeyOnly(adminKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminKey == "" {
			AbortForbidden(c, "admin_not_configured", "admin not configured")
			return
		}
		key := c.GetHeader(HeaderAdminKey)
		if key == "" || key != adminKey {
			AbortForbidden(c, "forbidden", "forbidden")
			return
		}
		setAdminActor(c, &admin.Actor{
			Subject:  "legacy-admin",
			ActorID:  "admin",
			Role:     admin.RoleSuper,
			AuthMode: "legacy_key",
			Env:      "test",
		})
		c.Next()
	}
}

// RequireAdminPermission 要求当前管理 actor 拥有权限；失败写 deny 审计。
func RequireAdminPermission(perm string, auditor *admin.ActionAuditor) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetAdminActor(c)
		if actor == nil {
			AbortForbidden(c, "admin_actor_missing", "admin actor missing")
			return
		}
		reason := GetAdminReason(c)
		if err := admin.Require(actor.Role, perm); err != nil {
			if auditor != nil {
				_, _ = auditor.Record(admin.ActionInput{
					Actor:     *actor,
					Action:    perm,
					Resource:  c.FullPath(),
					Reason:    reason,
					RequestID: GetRequestID(c),
					Outcome:   "deny",
				})
			}
			AbortForbidden(c, "admin_permission_denied", "insufficient admin permission")
			return
		}
		c.Next()
	}
}

// AdminActionAudit 在 handler 完成后记录审计。
func AdminActionAudit(action string, auditor *admin.ActionAuditor) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if auditor == nil {
			return
		}
		actor := GetAdminActor(c)
		if actor == nil {
			return
		}
		outcome := "ok"
		if c.Writer.Status() >= 400 {
			outcome = "error"
		}
		_, _ = auditor.Record(admin.ActionInput{
			Actor:     *actor,
			Action:    action,
			Resource:  c.FullPath(),
			Reason:    GetAdminReason(c),
			RequestID: GetRequestID(c),
			Outcome:   outcome,
			Metadata: map[string]any{
				"status": c.Writer.Status(),
				"method": c.Request.Method,
				"path":   c.Request.URL.Path,
			},
		})
	}
}

// OptionalAdminAuth 尝试解析 Admin JWT / break-glass，失败不中断（用于 token 签发 bootstrap）。
func OptionalAdminAuth(cfg AdminAuthConfig) gin.HandlerFunc {
	inner := AdminAuthRBAC(cfg)
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		key := strings.TrimSpace(c.GetHeader(HeaderAdminKey))
		if authHeader == "" && key == "" {
			c.Next()
			return
		}
		// 有凭证则严格校验
		inner(c)
	}
}

func setAdminActor(c *gin.Context, actor *admin.Actor) {
	if actor == nil {
		return
	}
	c.Set(ContextKeyAdminActor, actor)
	c.Set(ContextKeyRole, actor.Role)
}

// GetAdminActor 提取管理 Actor。
func GetAdminActor(c *gin.Context) *admin.Actor {
	v, ok := c.Get(ContextKeyAdminActor)
	if !ok || v == nil {
		return nil
	}
	if a, ok := v.(*admin.Actor); ok {
		return a
	}
	return nil
}

// GetAdminReason 提取操作原因。
func GetAdminReason(c *gin.Context) string {
	if v, ok := c.Get("admin_reason"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return strings.TrimSpace(c.GetHeader(HeaderAdminReason))
}

func isWriteMethod(m string) bool {
	switch strings.ToUpper(m) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

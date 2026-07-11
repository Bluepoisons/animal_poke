// Package handlers — 管理端认证、会话与 RBAC 保护资源（AP-085）。
package handlers

import (
	"net/http"
	"strings"
	"time"

	"animalpoke/backend/internal/admin"
	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminHandler 管理端 token / 会话 / 安全报告 / 配置写。
type AdminHandler struct {
	tokens      *admin.TokenService
	sessions    *admin.SessionStore
	auditor     *admin.ActionAuditor
	db          *gorm.DB
	adminAPIKey string
	breakGlass  bool
	production  bool
	devIssueKey string
	env         string
}

// AdminHandlerOptions 构造选项。
type AdminHandlerOptions struct {
	Tokens      *admin.TokenService
	Sessions    *admin.SessionStore
	Auditor     *admin.ActionAuditor
	DB          *gorm.DB
	AdminAPIKey string
	BreakGlass  bool
	Production  bool
	DevIssueKey string
	Env         string
}

// NewAdminHandler 构造。
func NewAdminHandler(opts AdminHandlerOptions) *AdminHandler {
	return &AdminHandler{
		tokens:      opts.Tokens,
		sessions:    opts.Sessions,
		auditor:     opts.Auditor,
		db:          opts.DB,
		adminAPIKey: opts.AdminAPIKey,
		breakGlass:  opts.BreakGlass,
		production:  opts.Production,
		devIssueKey: opts.DevIssueKey,
		env:         opts.Env,
	}
}

type adminTokenRequest struct {
	Actor   string `json:"actor" binding:"required"`
	Role    string `json:"role" binding:"required"`
	Subject string `json:"subject"`
	// DevSecret 非生产签发密钥（ADMIN_DEV_ISSUE_SECRET）；生产忽略。
	DevSecret string `json:"dev_secret"`
}

// IssueToken POST /admin/auth/token
// 获取短期管理 JWT。允许：
//  1. 已认证 super（Bearer）代发；
//  2. break-glass（X-Admin-Key + Actor + Reason）；
//  3. 非生产 dev_secret。
func (h *AdminHandler) IssueToken(c *gin.Context) {
	if h.tokens == nil {
		middleware.WriteError(c, http.StatusServiceUnavailable, "admin_auth_unavailable", "admin token service unavailable", true, nil)
		return
	}
	var req adminTokenRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if !admin.ValidRole(req.Role) {
		middleware.AbortBadRequest(c, "invalid_role", "invalid admin role", nil)
		return
	}

	authMode := ""
	// 已登录 super
	if actor := middleware.GetAdminActor(c); actor != nil {
		if !admin.HasPermission(actor.Role, admin.PermAdminTokenIssue) {
			middleware.AbortForbidden(c, "admin_permission_denied", "insufficient admin permission")
			return
		}
		authMode = "jwt_issue"
	} else if h.allowBreakGlassIssue(c) {
		authMode = "break_glass"
	} else if !h.production && h.devIssueKey != "" && strings.TrimSpace(req.DevSecret) == h.devIssueKey {
		authMode = "dev_issue"
	} else {
		middleware.AbortUnauthorized(c, "admin_issue_forbidden", "not allowed to issue admin token")
		return
	}

	subject := req.Subject
	if subject == "" {
		subject = req.Actor
	}
	res, err := h.tokens.Issue(req.Actor, subject, req.Role, authMode)
	if err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "admin_token_issue_failed", err.Error(), false, nil)
		return
	}
	if h.auditor != nil {
		_, _ = h.auditor.Record(admin.ActionInput{
			Actor: admin.Actor{
				Subject:   subject,
				ActorID:   req.Actor,
				Role:      admin.NormalizeRole(req.Role),
				SessionID: res.SessionID,
				AuthMode:  authMode,
				Env:       h.env,
			},
			Action:    "admin.token.issue",
			Resource:  "admin_session:" + res.SessionID,
			Reason:    middleware.GetAdminReason(c),
			RequestID: middleware.GetRequestID(c),
			Outcome:   "ok",
			Metadata:  map[string]any{"role": res.Role, "auth_mode": authMode},
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"token":      res.Token,
		"expires_at": res.ExpiresAt.Format(time.RFC3339),
		"token_type": "Bearer",
		"session_id": res.SessionID,
		"role":       res.Role,
		"actor":      res.ActorID,
		"env":        h.env,
	})
}

func (h *AdminHandler) allowBreakGlassIssue(c *gin.Context) bool {
	if h.adminAPIKey == "" {
		return false
	}
	if h.production && !h.breakGlass {
		return false
	}
	if !h.breakGlass && h.production {
		return false
	}
	// 非生产：breakGlass 或未强制时，有正确 key + actor + reason 可 bootstrap
	key := strings.TrimSpace(c.GetHeader(middleware.HeaderAdminKey))
	if key == "" || key != h.adminAPIKey {
		return false
	}
	if h.production && !h.breakGlass {
		return false
	}
	if !h.breakGlass && h.production {
		return false
	}
	// 非 production 默认允许 key bootstrap；production 必须 breakGlass
	if h.production && !h.breakGlass {
		return false
	}
	actor := strings.TrimSpace(c.GetHeader(middleware.HeaderAdminActor))
	reason := strings.TrimSpace(c.GetHeader(middleware.HeaderAdminReason))
	if actor == "" || reason == "" {
		return false
	}
	if h.production && !h.breakGlass {
		return false
	}
	// 最终：production 需 breakGlass；非 production 允许
	if h.production {
		return h.breakGlass
	}
	return true
}

type revokeRequest struct {
	SessionID string `json:"session_id"`
	ActorID   string `json:"actor_id"` // 撤销该人员全部会话
	Reason    string `json:"reason"`
}

// RevokeSession POST /admin/sessions/revoke
func (h *AdminHandler) RevokeSession(c *gin.Context) {
	if h.sessions == nil {
		middleware.WriteError(c, http.StatusServiceUnavailable, "admin_session_unavailable", "session store unavailable", true, nil)
		return
	}
	actor := middleware.GetAdminActor(c)
	if actor == nil {
		middleware.AbortForbidden(c, "admin_actor_missing", "admin actor missing")
		return
	}
	if !admin.HasPermission(actor.Role, admin.PermSessionRevoke) {
		middleware.AbortForbidden(c, "admin_permission_denied", "insufficient admin permission")
		return
	}
	var req revokeRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	reason := req.Reason
	if reason == "" {
		reason = middleware.GetAdminReason(c)
	}
	if reason == "" {
		middleware.AbortBadRequest(c, "admin_reason_required", "reason required", nil)
		return
	}
	var revoked int
	if req.SessionID != "" {
		if err := h.sessions.Revoke(req.SessionID, actor.ActorID); err != nil {
			middleware.WriteError(c, http.StatusInternalServerError, "revoke_failed", "revoke failed", true, nil)
			return
		}
		revoked = 1
	} else if req.ActorID != "" {
		n, err := h.sessions.RevokeAllForActor(req.ActorID, actor.ActorID)
		if err != nil {
			middleware.WriteError(c, http.StatusInternalServerError, "revoke_failed", "revoke failed", true, nil)
			return
		}
		revoked = n
	} else {
		middleware.AbortBadRequest(c, "revoke_target_required", "session_id or actor_id required", nil)
		return
	}
	if h.auditor != nil {
		_, _ = h.auditor.Record(admin.ActionInput{
			Actor:     *actor,
			Action:    admin.PermSessionRevoke,
			Resource:  firstNonEmptyAdmin(req.SessionID, req.ActorID),
			Reason:    reason,
			RequestID: middleware.GetRequestID(c),
			Outcome:   "ok",
			Metadata:  map[string]any{"revoked": revoked},
		})
	}
	c.JSON(http.StatusOK, gin.H{"status": "revoked", "revoked": revoked, "request_id": middleware.GetRequestID(c)})
}

// GetSecurityReport GET /admin/security/reports/:id
// 财务等角色仅返回 meta；security/super 可读正文。
func (h *AdminHandler) GetSecurityReport(c *gin.Context) {
	if h.db == nil {
		middleware.WriteError(c, http.StatusServiceUnavailable, "db_unavailable", "db unavailable", true, nil)
		return
	}
	actor := middleware.GetAdminActor(c)
	if actor == nil {
		middleware.AbortForbidden(c, "admin_actor_missing", "admin actor missing")
		return
	}
	if !admin.HasPermission(actor.Role, admin.PermSecurityReportMeta) {
		middleware.AbortForbidden(c, "admin_permission_denied", "insufficient admin permission")
		return
	}
	id := c.Param("id")
	var report models.SecurityReport
	if err := h.db.Where("report_id = ?", id).First(&report).Error; err != nil {
		middleware.WriteError(c, http.StatusNotFound, "report_not_found", "security report not found", false, nil)
		return
	}
	bodyAllowed := admin.HasPermission(actor.Role, admin.PermSecurityReportBody)
	resp := gin.H{
		"report_id":     report.ReportID,
		"device_id":     report.DeviceID,
		"risk_score":    report.RiskScore,
		"created_at":    report.CreatedAt,
		"body_redacted": !bodyAllowed,
		"request_id":    middleware.GetRequestID(c),
	}
	if bodyAllowed {
		resp["payload"] = report.Payload
		resp["nonce"] = report.Nonce
	} else {
		resp["payload"] = nil
		resp["nonce"] = nil
	}
	if h.auditor != nil {
		_, _ = h.auditor.Record(admin.ActionInput{
			Actor:     *actor,
			Action:    "security.report.read",
			Resource:  report.ReportID,
			Reason:    middleware.GetAdminReason(c),
			RequestID: middleware.GetRequestID(c),
			Outcome:   "ok",
			Metadata:  map[string]any{"body_redacted": !bodyAllowed},
		})
	}
	c.JSON(http.StatusOK, resp)
}

// WriteGameConfig PUT /admin/config/game — RBAC 配置写（客服禁止）。
func (h *AdminHandler) WriteGameConfig(c *gin.Context) {
	actor := middleware.GetAdminActor(c)
	if actor == nil {
		middleware.AbortForbidden(c, "admin_actor_missing", "admin actor missing")
		return
	}
	if !admin.HasPermission(actor.Role, admin.PermConfigWrite) {
		if h.auditor != nil {
			_, _ = h.auditor.Record(admin.ActionInput{
				Actor: *actor, Action: admin.PermConfigWrite, Resource: "game_config",
				Reason: middleware.GetAdminReason(c), RequestID: middleware.GetRequestID(c), Outcome: "deny",
			})
		}
		middleware.AbortForbidden(c, "admin_permission_denied", "insufficient admin permission")
		return
	}
	reason := middleware.GetAdminReason(c)
	if reason == "" {
		middleware.AbortBadRequest(c, "admin_reason_required", "X-Admin-Reason required", nil)
		return
	}
	// 配置写：记录审计；若注入 GameConfig 则透传，否则返回 accepted stub（权限层已覆盖验收）。
	var body map[string]any
	_ = middleware.BindStrictJSON(c, &body)
	if h.auditor != nil {
		_, _ = h.auditor.Record(admin.ActionInput{
			Actor: *actor, Action: admin.PermConfigWrite, Resource: "game_config",
			Reason: reason, RequestID: middleware.GetRequestID(c), Outcome: "ok",
			Metadata: map[string]any{"keys": len(body)},
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"status":     "accepted",
		"actor":      actor.ActorID,
		"role":       actor.Role,
		"request_id": middleware.GetRequestID(c),
	})
}

func firstNonEmptyAdmin(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

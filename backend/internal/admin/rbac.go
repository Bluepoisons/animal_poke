// Package admin 管理端 RBAC：角色权限矩阵、短期 JWT、会话撤销与动作审计（AP-085）。
package admin

import (
	"fmt"
	"strings"
)

// 管理角色（客服 / 内容 / 运营 / 财务 / 安全 / 超级管理员）。
const (
	RoleSupport  = "support"
	RoleContent  = "content"
	RoleOps      = "ops"
	RoleFinance  = "finance"
	RoleSecurity = "security"
	RoleSuper    = "super"
)

// 权限点。
const (
	PermAuditLogsRead        = "audit.logs.read"
	PermAuditLogsAck         = "audit.logs.ack"
	PermCommerceRefund       = "commerce.refund"
	PermConfigWrite          = "config.write"
	PermSecurityReportMeta   = "security.report.read_meta"
	PermSecurityReportBody   = "security.report.read_body"
	PermSessionRevoke        = "session.revoke"
	PermAdminTokenIssue      = "admin.token.issue"
	PermAdminActionAuditRead = "admin.action.audit.read"
)

// Actor 已认证的管理端身份。
type Actor struct {
	Subject   string // 稳定主体（OIDC sub / 邮箱）
	ActorID   string // 展示用人员标识
	Role      string
	SessionID string
	JTI       string
	Env       string
	AuthMode  string // jwt | break_glass
}

// AllRoles 返回全部合法角色。
func AllRoles() []string {
	return []string{RoleSupport, RoleContent, RoleOps, RoleFinance, RoleSecurity, RoleSuper}
}

// ValidRole 校验角色名。
func ValidRole(role string) bool {
	switch NormalizeRole(role) {
	case RoleSupport, RoleContent, RoleOps, RoleFinance, RoleSecurity, RoleSuper:
		return true
	default:
		return false
	}
}

// NormalizeRole 规范化角色。
func NormalizeRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

// rolePermissions 角色 → 权限集合。
// 验收：客服不能退款/改配置；财务不能读安全报告正文。
var rolePermissions = map[string]map[string]struct{}{
	RoleSupport: {
		PermAuditLogsRead:      {},
		PermSecurityReportMeta: {},
	},
	RoleContent: {
		PermAuditLogsRead: {},
		// 内容岗不碰财务/安全敏感操作
	},
	RoleOps: {
		PermAuditLogsRead:      {},
		PermAuditLogsAck:       {},
		PermConfigWrite:        {},
		PermSecurityReportMeta: {},
	},
	RoleFinance: {
		PermCommerceRefund:     {},
		PermSecurityReportMeta: {},
		// 明确禁止 security.report.read_body / config.write
	},
	RoleSecurity: {
		PermAuditLogsRead:        {},
		PermAuditLogsAck:         {},
		PermSecurityReportMeta:   {},
		PermSecurityReportBody:   {},
		PermSessionRevoke:        {},
		PermAdminActionAuditRead: {},
	},
	RoleSuper: {
		PermAuditLogsRead:        {},
		PermAuditLogsAck:         {},
		PermCommerceRefund:       {},
		PermConfigWrite:          {},
		PermSecurityReportMeta:   {},
		PermSecurityReportBody:   {},
		PermSessionRevoke:        {},
		PermAdminTokenIssue:      {},
		PermAdminActionAuditRead: {},
	},
}

// PermissionsFor 返回角色权限列表。
func PermissionsFor(role string) []string {
	role = NormalizeRole(role)
	set, ok := rolePermissions[role]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	return out
}

// HasPermission 判断角色是否拥有权限。
func HasPermission(role, perm string) bool {
	role = NormalizeRole(role)
	if role == RoleSuper {
		// super 拥有矩阵内全部权限；未知权限仍拒绝以防拼写漏洞
		if _, known := knownPermissions[perm]; known {
			return true
		}
		// super 映射表已列全；以表为准
	}
	set, ok := rolePermissions[role]
	if !ok {
		return false
	}
	_, ok = set[perm]
	return ok
}

var knownPermissions = map[string]struct{}{
	PermAuditLogsRead:        {},
	PermAuditLogsAck:         {},
	PermCommerceRefund:       {},
	PermConfigWrite:          {},
	PermSecurityReportMeta:   {},
	PermSecurityReportBody:   {},
	PermSessionRevoke:        {},
	PermAdminTokenIssue:      {},
	PermAdminActionAuditRead: {},
}

// Require 返回无权时的错误信息。
func Require(role, perm string) error {
	if HasPermission(role, perm) {
		return nil
	}
	return fmt.Errorf("role %s lacks permission %s", NormalizeRole(role), perm)
}

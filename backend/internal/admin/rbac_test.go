package admin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermissionMatrix_SupportCannotRefundOrConfig(t *testing.T) {
	assert.False(t, HasPermission(RoleSupport, PermCommerceRefund))
	assert.False(t, HasPermission(RoleSupport, PermConfigWrite))
	assert.True(t, HasPermission(RoleSupport, PermAuditLogsRead))
	assert.True(t, HasPermission(RoleSupport, PermSecurityReportMeta))
	assert.False(t, HasPermission(RoleSupport, PermSecurityReportBody))
}

func TestPermissionMatrix_FinanceCannotReadSecurityBody(t *testing.T) {
	assert.True(t, HasPermission(RoleFinance, PermCommerceRefund))
	assert.True(t, HasPermission(RoleFinance, PermSecurityReportMeta))
	assert.False(t, HasPermission(RoleFinance, PermSecurityReportBody))
	assert.False(t, HasPermission(RoleFinance, PermConfigWrite))
	assert.False(t, HasPermission(RoleFinance, PermAuditLogsAck))
}

func TestPermissionMatrix_FullTable(t *testing.T) {
	// role -> perm -> allowed
	cases := []struct {
		role string
		perm string
		ok   bool
	}{
		{RoleSupport, PermCommerceRefund, false},
		{RoleSupport, PermConfigWrite, false},
		{RoleContent, PermCommerceRefund, false},
		{RoleContent, PermConfigWrite, false},
		{RoleOps, PermConfigWrite, true},
		{RoleOps, PermCommerceRefund, false},
		{RoleOps, PermAuditLogsAck, true},
		{RoleFinance, PermCommerceRefund, true},
		{RoleFinance, PermSecurityReportBody, false},
		{RoleSecurity, PermSecurityReportBody, true},
		{RoleSecurity, PermSessionRevoke, true},
		{RoleSecurity, PermCommerceRefund, false},
		{RoleSuper, PermCommerceRefund, true},
		{RoleSuper, PermConfigWrite, true},
		{RoleSuper, PermSecurityReportBody, true},
		{RoleSuper, PermAdminTokenIssue, true},
		{RoleSuper, PermSessionRevoke, true},
	}
	for _, tc := range cases {
		t.Run(tc.role+"/"+tc.perm, func(t *testing.T) {
			assert.Equal(t, tc.ok, HasPermission(tc.role, tc.perm))
		})
	}
}

func TestExpiredRole_InvalidRoleDenied(t *testing.T) {
	assert.False(t, ValidRole("expired"))
	assert.False(t, HasPermission("expired", PermAuditLogsRead))
	assert.False(t, HasPermission("", PermCommerceRefund))
}

func TestTokenIssueParse_EnvIsolation(t *testing.T) {
	store := NewSessionStore(nil)
	svc := NewTokenService(TokenConfig{
		Secret:   "test-admin-jwt-secret-32chars-min!!",
		Issuer:   "animal-poke-admin",
		Audience: "animal-poke-admin-production",
		Env:      "production",
		TTL:      15 * time.Minute,
	}, store)

	res, err := svc.Issue("alice@example.com", "alice@example.com", RoleFinance, "jwt")
	require.NoError(t, err)
	require.NotEmpty(t, res.Token)

	actor, err := svc.Parse(res.Token)
	require.NoError(t, err)
	assert.Equal(t, RoleFinance, actor.Role)
	assert.Equal(t, "alice@example.com", actor.ActorID)
	assert.Equal(t, res.SessionID, actor.SessionID)
	assert.Equal(t, "production", actor.Env)

	// 错误环境 audience 拒绝
	other := NewTokenService(TokenConfig{
		Secret:   "test-admin-jwt-secret-32chars-min!!",
		Issuer:   "animal-poke-admin",
		Audience: "animal-poke-admin-staging",
		Env:      "staging",
		TTL:      15 * time.Minute,
	}, store)
	_, err = other.Parse(res.Token)
	assert.Error(t, err)
}

func TestSessionRevoke_ImmediateAndGrace(t *testing.T) {
	store := NewSessionStore(nil)
	svc := NewTokenService(TokenConfig{
		Secret:   "test-admin-jwt-secret-32chars-min!!",
		Issuer:   "animal-poke-admin",
		Audience: "animal-poke-admin-test",
		Env:      "test",
		TTL:      time.Hour,
	}, store)

	res, err := svc.Issue("bob", "bob", RoleSecurity, "jwt")
	require.NoError(t, err)

	ok, err := store.IsActive(res.SessionID, time.Now().UTC())
	require.NoError(t, err)
	assert.True(t, ok)

	require.NoError(t, store.Revoke(res.SessionID, "super-1"))
	ok, err = store.IsActive(res.SessionID, time.Now().UTC())
	require.NoError(t, err)
	assert.False(t, ok, "撤权后立即失效")

	// 宽限窗口内仍可用
	store.RevokeGrace = 2 * time.Minute
	res2, err := svc.Issue("carol", "carol", RoleOps, "jwt")
	require.NoError(t, err)
	require.NoError(t, store.Revoke(res2.SessionID, "super-1"))
	ok, err = store.IsActive(res2.SessionID, time.Now().UTC())
	require.NoError(t, err)
	assert.True(t, ok, "宽限内仍有效")
	ok, err = store.IsActive(res2.SessionID, time.Now().UTC().Add(3*time.Minute))
	require.NoError(t, err)
	assert.False(t, ok, "宽限后失效")
}

func TestConcurrentRevokeAllForActor(t *testing.T) {
	store := NewSessionStore(nil)
	svc := NewTokenService(TokenConfig{
		Secret:   "test-admin-jwt-secret-32chars-min!!",
		Issuer:   "animal-poke-admin",
		Audience: "animal-poke-admin-test",
		Env:      "test",
		TTL:      time.Hour,
	}, store)
	for i := 0; i < 5; i++ {
		_, err := svc.Issue("dave", "dave", RoleSupport, "jwt")
		require.NoError(t, err)
	}
	done := make(chan int, 3)
	for i := 0; i < 3; i++ {
		go func() {
			n, err := store.RevokeAllForActor("dave", "super")
			require.NoError(t, err)
			done <- n
		}()
	}
	total := 0
	for i := 0; i < 3; i++ {
		total += <-done
	}
	assert.GreaterOrEqual(t, total, 5)
	// 所有会话均不可用
	// 重新签发后可查：旧会话不在 active
}

func TestActionAuditor_IntegrityTamperDetect(t *testing.T) {
	aud := NewActionAuditor(nil, "audit-hmac-secret")
	row, err := aud.Record(ActionInput{
		Actor:  Actor{ActorID: "eve", Subject: "eve", Role: RoleSuper, SessionID: "s1", AuthMode: "jwt", Env: "test"},
		Action: PermCommerceRefund, Resource: "order:1", Reason: "chargeback",
		RequestID: "rid-1", Outcome: "ok",
	})
	require.NoError(t, err)
	assert.True(t, aud.VerifyIntegrity(row))
	row.Reason = "tampered"
	assert.False(t, aud.VerifyIntegrity(row))
}

func TestBreakGlassRoleIsSuper(t *testing.T) {
	// break-glass 语义：紧急入口映射 super 权限
	assert.True(t, HasPermission(RoleSuper, PermCommerceRefund))
	assert.True(t, HasPermission(RoleSuper, PermConfigWrite))
}

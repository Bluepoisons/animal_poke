package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"animalpoke/backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AP-079: 新邮箱绑定为 pending，未验证不可登录恢复。
func TestAP079_UnverifiedEmailCannotLogin(t *testing.T) {
	r, db, _, _ := setupAccountTest(t)
	token := deviceAuth(t, r, "dev-pending-1")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "email", "email": "pending@example.com", "password": "password123",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var bind map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bind))
	assert.Equal(t, false, bind["email_verified"])
	assert.Equal(t, true, bind["verification_required"])
	assert.NotEmpty(t, bind["debug_security_token"])

	var b models.AccountBinding
	require.NoError(t, db.Where("provider_subject = ?", "pending@example.com").First(&b).Error)
	assert.False(t, b.Verified)

	// 未验证不可 login
	w2 := postLogin(t, r, map[string]string{
		"device_id": "dev-pending-2", "provider": "email",
		"email": "pending@example.com", "password": "password123",
	})
	require.Equal(t, http.StatusUnauthorized, w2.Code, w2.Body.String())
	assert.Contains(t, w2.Body.String(), "auth_failed")

	// 验证后可 login
	w3 := authedJSON(t, r, "POST", "/api/v1/auth/email/verify", "", map[string]string{
		"token": bind["debug_security_token"].(string),
	})
	require.Equal(t, 200, w3.Code, w3.Body.String())

	w4 := postLogin(t, r, map[string]string{
		"device_id": "dev-pending-2", "provider": "email",
		"email": "pending@example.com", "password": "password123",
	})
	require.Equal(t, 200, w4.Code, w4.Body.String())
}

// AP-079: token 重放拒绝。
func TestAP079_EmailVerifyTokenReplay(t *testing.T) {
	r, _, _, _ := setupAccountTest(t)
	token := deviceAuth(t, r, "dev-replay-1")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "email", "email": "replay@example.com", "password": "password123",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var bind map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bind))
	sec := bind["debug_security_token"].(string)

	w1 := authedJSON(t, r, "POST", "/api/v1/auth/email/verify", "", map[string]string{"token": sec})
	require.Equal(t, 200, w1.Code, w1.Body.String())
	w2 := authedJSON(t, r, "POST", "/api/v1/auth/email/verify", "", map[string]string{"token": sec})
	require.Equal(t, http.StatusConflict, w2.Code, w2.Body.String())
	assert.Contains(t, w2.Body.String(), "token_replay")
}

// AP-079: 并发验证仅一次成功。
func TestAP079_EmailVerifyConcurrentOnce(t *testing.T) {
	r, _, _, _ := setupAccountTest(t)
	token := deviceAuth(t, r, "dev-conc-verify")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "email", "email": "conc-v@example.com", "password": "password123",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var bind map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bind))
	sec := bind["debug_security_token"].(string)

	const n = 8
	codes := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			ww := authedJSON(t, r, "POST", "/api/v1/auth/email/verify", "", map[string]string{"token": sec})
			codes[i] = ww.Code
		}(i)
	}
	wg.Wait()
	ok, fail := 0, 0
	for _, c := range codes {
		if c == 200 {
			ok++
		} else {
			fail++
		}
	}
	assert.Equal(t, 1, ok)
	assert.Equal(t, n-1, fail)
}

// AP-079: 改密吊销全部 access/refresh。
func TestAP079_ChangePasswordInvalidatesSessions(t *testing.T) {
	r, _, deviceRepo, _ := setupAccountTest(t)
	access, refresh, _ := bindAndGetRefresh(t, r, "dev-chpw-1", "chpw@example.com")

	// 第二台设备也登录
	wLogin := postLogin(t, r, map[string]string{
		"device_id": "dev-chpw-2", "provider": "email",
		"email": "chpw@example.com", "password": "password123",
	})
	require.Equal(t, 200, wLogin.Code, wLogin.Body.String())
	var login2 accountAuthResponse
	require.NoError(t, json.Unmarshal(wLogin.Body.Bytes(), &login2))

	// 改密
	w := authedJSON(t, r, "POST", "/api/v1/auth/password/change", access, map[string]string{
		"current_password": "password123",
		"new_password":     "newpassword99",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var changed accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &changed))
	require.NotEmpty(t, changed.Token)
	require.NotEmpty(t, changed.RefreshToken)

	// 旧 access 失效
	wOld := authedJSON(t, r, "GET", "/api/v1/auth/account", access, nil)
	assert.Equal(t, 401, wOld.Code)
	wOld2 := authedJSON(t, r, "GET", "/api/v1/auth/account", login2.Token, nil)
	assert.Equal(t, 401, wOld2.Code)

	// 旧 refresh 失效
	wRef := authedJSON(t, r, "POST", "/api/v1/auth/refresh", "", map[string]string{
		"refresh_token": refresh,
	})
	assert.Equal(t, 401, wRef.Code)

	// 新 token 可用
	wNew := authedJSON(t, r, "GET", "/api/v1/auth/account", changed.Token, nil)
	require.Equal(t, 200, wNew.Code, wNew.Body.String())

	dev, err := deviceRepo.Find("dev-chpw-1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, dev.TokenVersion, 2)
}

// AP-079: 找回密码反枚举 + 仅已验证邮箱可重置。
func TestAP079_ForgotPasswordAntiEnumAndReset(t *testing.T) {
	r, _, _, _ := setupAccountTest(t)

	// 不存在邮箱：仍 200
	w0 := authedJSON(t, r, "POST", "/api/v1/auth/password/forgot", "", map[string]string{
		"email": "nobody@example.com",
	})
	require.Equal(t, 200, w0.Code, w0.Body.String())
	var body0 map[string]any
	require.NoError(t, json.Unmarshal(w0.Body.Bytes(), &body0))
	assert.Equal(t, "accepted", body0["status"])
	assert.Nil(t, body0["debug_security_token"])

	// 未验证邮箱：accepted 但不发 token
	token := deviceAuth(t, r, "dev-fp-pending")
	wBind := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "email", "email": "fp-pending@example.com", "password": "password123",
	})
	require.Equal(t, 200, wBind.Code)
	w1 := authedJSON(t, r, "POST", "/api/v1/auth/password/forgot", "", map[string]string{
		"email": "fp-pending@example.com",
	})
	require.Equal(t, 200, w1.Code)
	var body1 map[string]any
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &body1))
	assert.Equal(t, "accepted", body1["status"])
	assert.Nil(t, body1["debug_security_token"])

	// 已验证：可重置
	access, _, _ := bindAndGetRefresh(t, r, "dev-fp-ok", "fp-ok@example.com")
	_ = access
	w2 := authedJSON(t, r, "POST", "/api/v1/auth/password/forgot", "", map[string]string{
		"email": "fp-ok@example.com",
	})
	require.Equal(t, 200, w2.Code, w2.Body.String())
	var body2 map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &body2))
	resetTok, _ := body2["debug_security_token"].(string)
	require.NotEmpty(t, resetTok)

	w3 := authedJSON(t, r, "POST", "/api/v1/auth/password/reset", "", map[string]string{
		"token": resetTok, "new_password": "resetpass88",
	})
	require.Equal(t, 200, w3.Code, w3.Body.String())

	// 旧密码失败，新密码成功
	wOld := postLogin(t, r, map[string]string{
		"device_id": "dev-fp-login", "provider": "email",
		"email": "fp-ok@example.com", "password": "password123",
	})
	assert.Equal(t, 401, wOld.Code)
	wNew := postLogin(t, r, map[string]string{
		"device_id": "dev-fp-login", "provider": "email",
		"email": "fp-ok@example.com", "password": "resetpass88",
	})
	require.Equal(t, 200, wNew.Code, wNew.Body.String())

	// 重放 reset token
	wReplay := authedJSON(t, r, "POST", "/api/v1/auth/password/reset", "", map[string]string{
		"token": resetTok, "new_password": "anotherpass1",
	})
	assert.Equal(t, http.StatusConflict, wReplay.Code)
}

// AP-079: 解绑最后恢复方式被拒；可解绑多余 OAuth。
func TestAP079_UnbindLastRecoveryProtected(t *testing.T) {
	r, _, _, _ := setupAccountTest(t)
	token := deviceAuth(t, r, "dev-unbind-1")
	w := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "email", "email": "unbind@example.com", "password": "password123",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	verifyEmailFromBind(t, r, w.Body.Bytes())
	var bind accountAuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bind))
	token = bind.Token

	// 仅邮箱：解绑拒绝
	w1 := authedJSON(t, r, "POST", "/api/v1/auth/unbind", token, map[string]string{
		"provider": "email", "reauth_password": "password123",
	})
	require.Equal(t, http.StatusConflict, w1.Code, w1.Body.String())
	assert.Contains(t, w1.Body.String(), "last_recovery_method")

	// 再绑 OAuth 后可解绑 email
	w2 := authedJSON(t, r, "POST", "/api/v1/auth/bind", token, map[string]string{
		"provider": "mock_oauth", "oauth_subject": "unbind-oauth", "oauth_token": "oauth-token-xyz",
	})
	require.Equal(t, 200, w2.Code, w2.Body.String())
	var bind2 accountAuthResponse
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &bind2))
	token = bind2.Token

	w3 := authedJSON(t, r, "POST", "/api/v1/auth/unbind", token, map[string]string{
		"provider": "email", "reauth_password": "password123",
	})
	require.Equal(t, 200, w3.Code, w3.Body.String())

	// 剩余 oauth 不可再解绑
	w4 := authedJSON(t, r, "POST", "/api/v1/auth/unbind", token, map[string]string{
		"provider": "mock_oauth", "subject": "unbind-oauth", "reauth_password": "password123",
	})
	// reauth uses verified email — already unbound, so reauth fails OR last recovery
	// 无 verified email 后 reauth_password 失败
	assert.True(t, w4.Code == http.StatusForbidden || w4.Code == http.StatusConflict, w4.Body.String())
}

// AP-079: reauth 签发短期令牌。
func TestAP079_ReauthToken(t *testing.T) {
	r, _, _, _ := setupAccountTest(t)
	access, _, _ := bindAndGetRefresh(t, r, "dev-reauth-1", "reauth@example.com")

	w := authedJSON(t, r, "POST", "/api/v1/auth/reauth", access, map[string]string{
		"password": "password123",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["reauth_token"])
	assert.Equal(t, "reauth", resp["token_type"])

	// 错误密码
	wBad := authedJSON(t, r, "POST", "/api/v1/auth/reauth", access, map[string]string{
		"password": "wrong-password",
	})
	assert.Equal(t, 401, wBad.Code)
}

// AP-079: 错误密码与不存在账号响应不可区分。
func TestAP079_LoginAntiEnumeration(t *testing.T) {
	r, _, _, _ := setupAccountTest(t)
	_, _, _ = bindAndGetRefresh(t, r, "dev-enum-1", "enum@example.com")

	wMissing := postLogin(t, r, map[string]string{
		"device_id": "dev-enum-x", "provider": "email",
		"email": "missing@example.com", "password": "password123",
	})
	wWrong := postLogin(t, r, map[string]string{
		"device_id": "dev-enum-x", "provider": "email",
		"email": "enum@example.com", "password": "wrong-password",
	})
	require.Equal(t, wMissing.Code, wWrong.Code)
	var a, b map[string]any
	require.NoError(t, json.Unmarshal(wMissing.Body.Bytes(), &a))
	require.NoError(t, json.Unmarshal(wWrong.Body.Bytes(), &b))
	assert.Equal(t, a["reason_code"], b["reason_code"])
	assert.Equal(t, a["error"], b["error"])
}

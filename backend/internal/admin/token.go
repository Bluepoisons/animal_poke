package admin

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenConfig 管理端 JWT 配置（与设备 JWT 隔离：独立 iss/aud/密钥）。
type TokenConfig struct {
	Secret         string
	PreviousSecret string
	Issuer         string
	// Audience 含环境隔离，如 animal-poke-admin-production。
	Audience string
	Env      string
	TTL      time.Duration
}

// TokenService 签发/校验短期管理 JWT。
type TokenService struct {
	cfg      TokenConfig
	sessions *SessionStore
}

// NewTokenService 构造。
func NewTokenService(cfg TokenConfig, sessions *SessionStore) *TokenService {
	if cfg.TTL <= 0 {
		cfg.TTL = 15 * time.Minute
	}
	if cfg.Issuer == "" {
		cfg.Issuer = "animal-poke-admin"
	}
	if cfg.Audience == "" {
		cfg.Audience = "animal-poke-admin-" + cfg.Env
	}
	return &TokenService{cfg: cfg, sessions: sessions}
}

// IssueResult 签发结果。
type IssueResult struct {
	Token     string
	ExpiresAt time.Time
	SessionID string
	Role      string
	ActorID   string
}

// Issue 创建会话并签发短期 Admin JWT。
func (t *TokenService) Issue(actorID, subject, role, authMode string) (*IssueResult, error) {
	if t == nil || t.cfg.Secret == "" {
		return nil, fmt.Errorf("admin token service not configured")
	}
	role = NormalizeRole(role)
	if !ValidRole(role) {
		return nil, fmt.Errorf("invalid role")
	}
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return nil, fmt.Errorf("actor required")
	}
	if subject == "" {
		subject = actorID
	}
	if authMode == "" {
		authMode = "jwt"
	}
	var sessID string
	if t.sessions != nil {
		sess, err := t.sessions.Create(actorID, subject, role, t.cfg.Env, authMode, t.cfg.TTL)
		if err != nil {
			return nil, err
		}
		sessID = sess.SessionID
	} else {
		sessID = uuid.NewString()
	}
	now := time.Now().UTC()
	exp := now.Add(t.cfg.TTL)
	jti := uuid.NewString()
	claims := jwt.MapClaims{
		"sub":       subject,
		"actor":     actorID,
		"role":      role,
		"sid":       sessID,
		"iss":       t.cfg.Issuer,
		"aud":       t.cfg.Audience,
		"env":       t.cfg.Env,
		"jti":       jti,
		"iat":       now.Unix(),
		"exp":       exp.Unix(),
		"token_use": "admin_access",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = "admin-v1"
	signed, err := token.SignedString([]byte(t.cfg.Secret))
	if err != nil {
		return nil, err
	}
	return &IssueResult{
		Token:     signed,
		ExpiresAt: exp,
		SessionID: sessID,
		Role:      role,
		ActorID:   actorID,
	}, nil
}

// Parse 校验 Admin JWT 并返回 Actor（不检查会话撤销；由中间件再查 SessionStore）。
func (t *TokenService) Parse(tokenStr string) (*Actor, error) {
	if t == nil || t.cfg.Secret == "" {
		return nil, fmt.Errorf("admin token service not configured")
	}
	secrets := []string{t.cfg.Secret}
	if t.cfg.PreviousSecret != "" && t.cfg.PreviousSecret != t.cfg.Secret {
		secrets = append(secrets, t.cfg.PreviousSecret)
	}
	var lastErr error
	for _, secret := range secrets {
		token, err := jwt.Parse(tokenStr, func(tok *jwt.Token) (interface{}, error) {
			if tok.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
		if err != nil || token == nil || !token.Valid {
			lastErr = err
			continue
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, fmt.Errorf("invalid claims")
		}
		if use, _ := claims["token_use"].(string); use != "admin_access" {
			return nil, fmt.Errorf("invalid token_use")
		}
		iss, _ := claims["iss"].(string)
		if iss == "" || iss != t.cfg.Issuer {
			return nil, fmt.Errorf("invalid issuer")
		}
		if !audienceOK(claims["aud"], t.cfg.Audience) {
			return nil, fmt.Errorf("invalid audience")
		}
		env, _ := claims["env"].(string)
		if t.cfg.Env != "" && env != "" && env != t.cfg.Env {
			return nil, fmt.Errorf("env isolation mismatch")
		}
		role, _ := claims["role"].(string)
		role = NormalizeRole(role)
		if !ValidRole(role) {
			return nil, fmt.Errorf("invalid role")
		}
		actorID, _ := claims["actor"].(string)
		if actorID == "" {
			actorID, _ = claims["sub"].(string)
		}
		if actorID == "" {
			return nil, fmt.Errorf("actor required")
		}
		sub, _ := claims["sub"].(string)
		if sub == "" {
			sub = actorID
		}
		sid, _ := claims["sid"].(string)
		jti, _ := claims["jti"].(string)
		if jti == "" {
			return nil, fmt.Errorf("jti required")
		}
		return &Actor{
			Subject:   sub,
			ActorID:   actorID,
			Role:      role,
			SessionID: sid,
			JTI:       jti,
			Env:       env,
			AuthMode:  "jwt",
		}, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("invalid admin token")
}

func audienceOK(claim interface{}, expected string) bool {
	switch aud := claim.(type) {
	case string:
		return aud != "" && aud == expected
	case []interface{}:
		for _, a := range aud {
			if s, ok := a.(string); ok && s == expected {
				return true
			}
		}
	case []string:
		for _, s := range aud {
			if s == expected {
				return true
			}
		}
	}
	return false
}

// Package services 邮件投递抽象（AP-079 账号安全）。
package services

import (
	"log/slog"
	"sync"
)

// SecurityMailer 投递验证/找回等安全邮件。生产可接 SMTP/SES；开发默认日志。
type SecurityMailer interface {
	// SendSecurityMail purpose=email_verify|password_reset；token 为明文一次性令牌。
	SendSecurityMail(to, purpose, plainToken string) error
}

// LogSecurityMailer 将邮件内容写日志（不含生产投递）；开发/测试默认。
type LogSecurityMailer struct{}

// SendSecurityMail 实现 SecurityMailer。
func (LogSecurityMailer) SendSecurityMail(to, purpose, plainToken string) error {
	slog.Info("security mail (log sink)", "to", to, "purpose", purpose, "token_len", len(plainToken))
	return nil
}

// MemorySecurityMailer 测试用：捕获最近投递。
type MemorySecurityMailer struct {
	mu    sync.Mutex
	Mails []SecurityMail
}

// SecurityMail 捕获的一封安全邮件。
type SecurityMail struct {
	To      string
	Purpose string
	Token   string
}

// SendSecurityMail 实现 SecurityMailer。
func (m *MemorySecurityMailer) SendSecurityMail(to, purpose, plainToken string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Mails = append(m.Mails, SecurityMail{To: to, Purpose: purpose, Token: plainToken})
	return nil
}

// LastToken 返回指定 purpose 最近一封明文令牌。
func (m *MemorySecurityMailer) LastToken(purpose string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.Mails) - 1; i >= 0; i-- {
		if purpose == "" || m.Mails[i].Purpose == purpose {
			return m.Mails[i].Token
		}
	}
	return ""
}

// Reset 清空捕获。
func (m *MemorySecurityMailer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Mails = nil
}

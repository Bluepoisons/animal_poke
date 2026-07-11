// Package cases implements AP-086 unified backoffice case workflow.
package cases

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// States
const (
	StateOpen       = "open"
	StateTriaged    = "triaged"
	StateInProgress = "in_progress"
	StateResolved   = "resolved"
	StateRejected   = "rejected"
	StateReopened   = "reopened"
)

// Resource kinds linked to a case.
const (
	ResourceModeration = "moderation"
	ResourcePrivacy    = "privacy"
	ResourceSecurity   = "security"
	ResourceAccount    = "account"
	ResourceOrder      = "order"
)

// Case is the unified case record.
type Case struct {
	ID           string    `json:"id"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	State        string    `json:"state"`
	Assignee     string    `json:"assignee,omitempty"`
	SLADueAt     time.Time `json:"sla_due_at"`
	SLABreached  bool      `json:"sla_breached"`
	Reason       string    `json:"reason,omitempty"`
	UserVisible  string    `json:"user_status"` // safe status for end users
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	// Sensitive fields — redacted for non-privileged roles.
	InternalNotes []Note   `json:"internal_notes,omitempty"`
	Attachments   []string `json:"attachments,omitempty"`
	ReporterEmail string   `json:"reporter_email,omitempty"`
}

// Note is an immutable case note.
type Note struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditEvent is an immutable view/modify audit row.
type AuditEvent struct {
	ID        string    `json:"id"`
	CaseID    string    `json:"case_id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"` // view|create|assign|transition|note|export
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Role controls field visibility.
type Role string

const (
	RoleSupport Role = "support"
	RoleAdmin   Role = "admin"
	RoleUser    Role = "user"
)

var allowedTransitions = map[string][]string{
	StateOpen:       {StateTriaged, StateRejected},
	StateTriaged:    {StateInProgress, StateRejected},
	StateInProgress: {StateResolved, StateRejected},
	StateResolved:   {StateReopened},
	StateRejected:   {StateReopened},
	StateReopened:   {StateTriaged, StateInProgress, StateRejected},
}

// Service is an in-memory case store (process-local, deterministic tests).
type Service struct {
	mu     sync.Mutex
	seq    int
	cases  map[string]*Case
	audits []AuditEvent
	now    func() time.Time
}

// NewService creates a case service.
func NewService() *Service {
	return &Service{
		cases: map[string]*Case{},
		now:   func() time.Time { return time.Now().UTC() },
	}
}

// SetClock overrides time.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Create opens a new case.
func (s *Service) Create(resourceType, resourceID, createdBy, reporterEmail string, slaHours int) (*Case, error) {
	if !validResource(resourceType) {
		return nil, errors.New("invalid resource_type")
	}
	if strings.TrimSpace(resourceID) == "" {
		return nil, errors.New("resource_id required")
	}
	if slaHours <= 0 {
		slaHours = 24
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	id := fmt.Sprintf("case-%06d", s.seq)
	now := s.now()
	c := &Case{
		ID: id, ResourceType: resourceType, ResourceID: resourceID,
		State: StateOpen, UserVisible: "open", CreatedBy: createdBy,
		ReporterEmail: reporterEmail, SLADueAt: now.Add(time.Duration(slaHours) * time.Hour),
		CreatedAt: now, UpdatedAt: now,
	}
	s.cases[id] = c
	s.auditLocked(id, createdBy, "create", resourceType+":"+resourceID)
	return redact(clone(c), RoleAdmin), nil
}

// Get returns a case for a role (404 if missing). Records view audit.
func (s *Service) Get(id, actor string, role Role) (*Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[id]
	if !ok {
		return nil, errNotFound
	}
	s.refreshSLALocked(c)
	s.auditLocked(id, actor, "view", string(role))
	return redact(clone(c), role), nil
}

// Transition changes state if allowed (409 otherwise).
func (s *Service) Transition(id, actor, to, reason string) (*Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[id]
	if !ok {
		return nil, errNotFound
	}
	s.refreshSLALocked(c)
	if !canTransition(c.State, to) {
		return nil, fmt.Errorf("%w: %s -> %s", errConflict, c.State, to)
	}
	from := c.State
	c.State = to
	c.UserVisible = userStatus(to)
	if reason != "" {
		c.Reason = reason
	}
	c.UpdatedAt = s.now()
	s.auditLocked(id, actor, "transition", from+"->"+to)
	return redact(clone(c), RoleAdmin), nil
}

// Assign claims/assigns a case. Concurrent claim: first writer wins.
func (s *Service) Assign(id, actor, assignee string) (*Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[id]
	if !ok {
		return nil, errNotFound
	}
	if c.Assignee != "" && c.Assignee != assignee {
		return nil, fmt.Errorf("%w: already assigned to %s", errConflict, c.Assignee)
	}
	c.Assignee = assignee
	if c.State == StateOpen {
		c.State = StateTriaged
		c.UserVisible = userStatus(StateTriaged)
	}
	c.UpdatedAt = s.now()
	s.auditLocked(id, actor, "assign", assignee)
	return redact(clone(c), RoleAdmin), nil
}

// AddNote appends an immutable internal note.
func (s *Service) AddNote(id, actor, body string) (*Case, error) {
	if strings.TrimSpace(body) == "" {
		return nil, errors.New("note body required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[id]
	if !ok {
		return nil, errNotFound
	}
	note := Note{ID: fmt.Sprintf("n-%d", len(c.InternalNotes)+1), Author: actor, Body: body, CreatedAt: s.now()}
	c.InternalNotes = append(c.InternalNotes, note)
	c.UpdatedAt = s.now()
	s.auditLocked(id, actor, "note", note.ID)
	return redact(clone(c), RoleAdmin), nil
}

// List returns cases (admin view), optionally filtered by state.
func (s *Service) List(state string, role Role) []Case {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Case, 0, len(s.cases))
	for _, c := range s.cases {
		s.refreshSLALocked(c)
		if state != "" && c.State != state {
			continue
		}
		out = append(out, *redact(clone(c), role))
	}
	return out
}

// UserStatus returns safe status for reporter (no internal notes).
func (s *Service) UserStatus(id, reporter string) (*Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[id]
	if !ok {
		return nil, errNotFound
	}
	// Only creator can query as user (simplified ACL).
	if c.CreatedBy != reporter && c.ReporterEmail != reporter {
		return nil, errNotFound
	}
	s.refreshSLALocked(c)
	s.auditLocked(id, reporter, "view", "user")
	return redact(clone(c), RoleUser), nil
}

// Audits returns immutable audit log for a case.
func (s *Service) Audits(caseID string) []AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AuditEvent, 0)
	for _, a := range s.audits {
		if a.CaseID == caseID {
			out = append(out, a)
		}
	}
	return out
}

func (s *Service) refreshSLALocked(c *Case) {
	if c.State == StateResolved || c.State == StateRejected {
		return
	}
	if s.now().After(c.SLADueAt) {
		c.SLABreached = true
	}
}

func (s *Service) auditLocked(caseID, actor, action, detail string) {
	s.audits = append(s.audits, AuditEvent{
		ID: fmt.Sprintf("a-%d", len(s.audits)+1), CaseID: caseID, Actor: actor,
		Action: action, Detail: detail, CreatedAt: s.now(),
	})
}

func canTransition(from, to string) bool {
	for _, t := range allowedTransitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

func validResource(r string) bool {
	switch r {
	case ResourceModeration, ResourcePrivacy, ResourceSecurity, ResourceAccount, ResourceOrder:
		return true
	default:
		return false
	}
}

func userStatus(state string) string {
	switch state {
	case StateOpen, StateTriaged, StateInProgress, StateReopened:
		return "processing"
	case StateResolved:
		return "resolved"
	case StateRejected:
		return "rejected"
	default:
		return state
	}
}

func clone(c *Case) *Case {
	cp := *c
	if c.InternalNotes != nil {
		cp.InternalNotes = append([]Note(nil), c.InternalNotes...)
	}
	if c.Attachments != nil {
		cp.Attachments = append([]string(nil), c.Attachments...)
	}
	return &cp
}

func redact(c *Case, role Role) *Case {
	switch role {
	case RoleUser:
		c.InternalNotes = nil
		c.Attachments = nil
		c.ReporterEmail = ""
		c.Assignee = ""
		c.SLABreached = false
		c.Reason = publicReason(c.Reason)
	case RoleSupport:
		// support sees notes but not full email
		if c.ReporterEmail != "" {
			c.ReporterEmail = maskEmail(c.ReporterEmail)
		}
	case RoleAdmin:
		// full
	}
	return c
}

func publicReason(r string) string {
	if r == "" {
		return ""
	}
	return "reviewed"
}

func maskEmail(e string) string {
	parts := strings.Split(e, "@")
	if len(parts) != 2 || len(parts[0]) == 0 {
		return "***"
	}
	return string(parts[0][0]) + "***@" + parts[1]
}

var (
	errNotFound = errors.New("case not found")
	errConflict = errors.New("invalid state transition")
)

// IsNotFound reports 404 class errors.
func IsNotFound(err error) bool { return errors.Is(err, errNotFound) }

// IsConflict reports 409 class errors.
func IsConflict(err error) bool {
	return errors.Is(err, errConflict) || (err != nil && strings.Contains(err.Error(), "invalid state")) || (err != nil && strings.Contains(err.Error(), "already assigned"))
}

var (
	defaultSvc  *Service
	defaultOnce sync.Once
)

// Default process-wide service.
func Default() *Service {
	defaultOnce.Do(func() { defaultSvc = NewService() })
	return defaultSvc
}

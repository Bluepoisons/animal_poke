// Package research implements AP-108 privacy-safe async collaboration.
// No chat, trade, or realtime location — invite code + coarse board only.
package research

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	MaxMembers        = 4
	DailyContribute   = 5
	GoalTargetDefault = 10
)

var (
	ErrNotFound      = errors.New("team not found")
	ErrForbidden     = errors.New("forbidden")
	ErrAlreadyMember = errors.New("already member")
	ErrFull          = errors.New("team full")
	ErrBlocked       = errors.New("blocked")
	ErrCap           = errors.New("daily contribution cap")
	ErrDuplicate     = errors.New("already claimed reward")
	ErrGoalOpen      = errors.New("goal not complete")
)

// Team is a small research squad.
type Team struct {
	ID        string
	Invite    string
	Title     string
	Owner     string
	Members   map[string]Member
	Blocks    map[string]map[string]bool // blocker -> blocked
	Board     []BoardItem                // coarse notes only
	Goal      int
	Progress  map[string]int // member -> total contributes
	DayKey    string
	DayCount  map[string]int
	Claimed   map[string]bool
	SoloMode  bool // equivalent single-player path
	CreatedAt time.Time
}

// Member public profile (no device/location/photos).
type Member struct {
	UserKey  string
	Nickname string // moderated plain label
	JoinedAt time.Time
}

// BoardItem is coarse research progress (no precise place).
type BoardItem struct {
	ID        string
	Author    string
	Region    string // city/region only
	Note      string // short non-PII
	CreatedAt time.Time
}

// Service process-local store.
type Service struct {
	mu    sync.Mutex
	teams map[string]*Team
	byInv map[string]string
	now   func() time.Time
}

func NewService() *Service {
	return &Service{
		teams: map[string]*Team{},
		byInv: map[string]string{},
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) SetClock(now func() time.Time) { s.now = now }

func (s *Service) Create(owner, nick, title string, solo bool) (*Team, error) {
	if owner == "" {
		return nil, errors.New("owner required")
	}
	nick = sanitizeNick(nick)
	s.mu.Lock()
	defer s.mu.Unlock()
	id := "team-" + randHex(4)
	inv := strings.ToUpper(randHex(3))
	t := &Team{
		ID: id, Invite: inv, Title: title, Owner: owner,
		Members:   map[string]Member{owner: {UserKey: owner, Nickname: nick, JoinedAt: s.now()}},
		Blocks:    map[string]map[string]bool{},
		Board:     nil,
		Goal:      GoalTargetDefault,
		Progress:  map[string]int{},
		DayKey:    dayKey(s.now()),
		DayCount:  map[string]int{},
		Claimed:   map[string]bool{},
		SoloMode:  solo,
		CreatedAt: s.now(),
	}
	s.teams[id] = t
	s.byInv[inv] = id
	return cloneTeam(t, owner), nil
}

func (s *Service) Join(invite, user, nick string) (*Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byInv[strings.ToUpper(strings.TrimSpace(invite))]
	if !ok {
		return nil, ErrNotFound
	}
	t := s.teams[id]
	if _, ok := t.Members[user]; ok {
		return nil, ErrAlreadyMember
	}
	if len(t.Members) >= MaxMembers {
		return nil, ErrFull
	}
	// blocked either way?
	for m := range t.Members {
		if t.Blocks[m][user] || t.Blocks[user][m] {
			return nil, ErrBlocked
		}
	}
	t.Members[user] = Member{UserKey: user, Nickname: sanitizeNick(nick), JoinedAt: s.now()}
	return cloneTeam(t, user), nil
}

func (s *Service) Leave(teamID, user string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.teams[teamID]
	if !ok {
		return ErrNotFound
	}
	if _, ok := t.Members[user]; !ok {
		return ErrForbidden
	}
	delete(t.Members, user)
	return nil
}

func (s *Service) Block(teamID, blocker, target string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.teams[teamID]
	if !ok {
		return ErrNotFound
	}
	if _, ok := t.Members[blocker]; !ok {
		return ErrForbidden
	}
	if t.Blocks[blocker] == nil {
		t.Blocks[blocker] = map[string]bool{}
	}
	t.Blocks[blocker][target] = true
	// remove target membership if present
	delete(t.Members, target)
	return nil
}

// Contribute adds async research progress (capped daily). No location/photos.
func (s *Service) Contribute(teamID, user, region, note string) (*Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.teams[teamID]
	if !ok {
		return nil, ErrNotFound
	}
	if _, ok := t.Members[user]; !ok {
		return nil, ErrForbidden
	}
	// blocked members cannot contribute
	for m := range t.Members {
		if t.Blocks[m][user] {
			return nil, ErrBlocked
		}
	}
	s.rollDayLocked(t)
	if t.DayCount[user] >= DailyContribute {
		return nil, ErrCap
	}
	t.DayCount[user]++
	t.Progress[user]++
	t.Board = append(t.Board, BoardItem{
		ID: fmt.Sprintf("b-%d", len(t.Board)+1), Author: user,
		Region: coarseRegion(region), Note: truncate(note, 80), CreatedAt: s.now(),
	})
	return cloneTeam(t, user), nil
}

// SoloContribute for equivalent single-player path.
func (s *Service) SoloContribute(teamID, user string) (*Team, error) {
	return s.Contribute(teamID, user, "home", "solo research note")
}

// ClaimReward once per member when total progress reaches goal.
func (s *Service) ClaimReward(teamID, user string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.teams[teamID]
	if !ok {
		return ErrNotFound
	}
	if _, ok := t.Members[user]; !ok {
		return ErrForbidden
	}
	if t.Claimed[user] {
		return ErrDuplicate
	}
	total := 0
	for _, v := range t.Progress {
		total += v
	}
	// Solo mode: personal goal; multi: shared goal
	need := t.Goal
	if t.SoloMode {
		need = t.Goal / 2
		if t.Progress[user] < need {
			return ErrGoalOpen
		}
	} else if total < need {
		return ErrGoalOpen
	}
	t.Claimed[user] = true
	return nil
}

func (s *Service) Get(teamID, viewer string) (*Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.teams[teamID]
	if !ok {
		return nil, ErrNotFound
	}
	if _, ok := t.Members[viewer]; !ok {
		return nil, ErrForbidden
	}
	return cloneTeam(t, viewer), nil
}

func (s *Service) rollDayLocked(t *Team) {
	k := dayKey(s.now())
	if t.DayKey != k {
		t.DayKey = k
		t.DayCount = map[string]int{}
	}
}

func cloneTeam(t *Team, viewer string) *Team {
	cp := *t
	cp.Members = map[string]Member{}
	for k, v := range t.Members {
		// hide blocked users from board authors? keep members list without device info
		if t.Blocks[viewer][k] {
			continue
		}
		cp.Members[k] = v
	}
	cp.Progress = map[string]int{}
	for k, v := range t.Progress {
		if _, ok := cp.Members[k]; ok || k == viewer {
			cp.Progress[k] = v
		}
	}
	cp.Board = append([]BoardItem(nil), t.Board...)
	// strip board items from blocked authors
	filtered := cp.Board[:0]
	for _, b := range cp.Board {
		if t.Blocks[viewer][b.Author] {
			continue
		}
		filtered = append(filtered, b)
	}
	cp.Board = filtered
	cp.Claimed = map[string]bool{}
	for k, v := range t.Claimed {
		cp.Claimed[k] = v
	}
	cp.Blocks = nil // never leak full block graph
	return &cp
}

func sanitizeNick(n string) string {
	n = strings.TrimSpace(n)
	if n == "" {
		return "researcher"
	}
	// crude abuse filter
	lower := strings.ToLower(n)
	for _, bad := range []string{"admin", "fuck", "shit"} {
		if strings.Contains(lower, bad) {
			return "researcher"
		}
	}
	if len(n) > 24 {
		return n[:24]
	}
	return n
}

func coarseRegion(r string) string {
	r = strings.TrimSpace(r)
	if r == "" {
		return "unknown"
	}
	if len(r) > 32 {
		return r[:32]
	}
	// reject if looks like coordinates
	if strings.ContainsAny(r, "0123456789") && (strings.Contains(r, ".") || strings.Contains(r, ",")) {
		return "redacted"
	}
	return r
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func dayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

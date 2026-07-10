package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// GameConfig is the versioned live-ops payload served to clients.
type GameConfig struct {
	Version  string                 `json:"version"`
	Economy  map[string]float64     `json:"economy"`
	Features map[string]bool        `json:"features"`
	Meta     map[string]interface{} `json:"meta,omitempty"`
}

// DefaultGameConfig matches frontend stamina/shop constants (AP-059).
func DefaultGameConfig() GameConfig {
	return GameConfig{
		Version: "game-config.v1",
		Economy: map[string]float64{
			"captureStaminaCost":     20,
			"dispatchStaminaCost":    20,
			"battleStaminaCost":      20,
			"staminaRecoveryPerHour": 10,
			"potionPrice":            150,
			"potionRecovery":         3,
			"toyBallPrice":           50,
			"premiumToyBallPrice":    120,
		},
		Features: map[string]bool{
			"achievements": true,
			"dispatch":     false,
			"ranking":      false,
			"pvp":          false,
			"social":       false,
			"ops":          false,
		},
		Meta: map[string]interface{}{
			"source": "default",
		},
	}
}

var (
	economyBounds = map[string][2]float64{
		"captureStaminaCost":     {1, 120},
		"dispatchStaminaCost":    {1, 120},
		"battleStaminaCost":      {1, 120},
		"staminaRecoveryPerHour": {1, 60},
		"potionPrice":            {1, 10000},
		"potionRecovery":         {1, 100},
		"toyBallPrice":           {1, 10000},
		"premiumToyBallPrice":    {1, 10000},
	}
)

// ValidateGameConfig returns error if any economy field is out of hard bounds.
func ValidateGameConfig(cfg GameConfig) error {
	if cfg.Version == "" {
		return errors.New("version required")
	}
	if cfg.Economy == nil {
		return errors.New("economy required")
	}
	for k, b := range economyBounds {
		v, ok := cfg.Economy[k]
		if !ok {
			return fmt.Errorf("economy.%s required", k)
		}
		if v < b[0] || v > b[1] {
			return fmt.Errorf("economy.%s out of bounds [%.0f,%.0f]: %v", k, b[0], b[1], v)
		}
	}
	return nil
}

// GameConfigStore keeps current + previous config for rollback without redeploy.
type GameConfigStore struct {
	mu       sync.RWMutex
	current  GameConfig
	previous *GameConfig
	path     string
}

func NewGameConfigStore(path string) *GameConfigStore {
	s := &GameConfigStore{current: DefaultGameConfig(), path: path}
	if path != "" {
		_ = s.loadFromDisk()
	}
	return s
}

func (s *GameConfigStore) Get() GameConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *GameConfigStore) Previous() *GameConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.previous
}

// Put validates and installs a new config; previous is retained for rollback.
func (s *GameConfigStore) Put(cfg GameConfig) error {
	if err := ValidateGameConfig(cfg); err != nil {
		return err
	}
	if cfg.Meta == nil {
		cfg.Meta = map[string]interface{}{}
	}
	cfg.Meta["source"] = "ops"
	s.mu.Lock()
	prev := s.current
	s.previous = &prev
	s.current = cfg
	path := s.path
	s.mu.Unlock()
	if path != "" {
		_ = s.persist(cfg)
	}
	return nil
}

// Rollback restores previous version if present.
func (s *GameConfigStore) Rollback() (GameConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.previous == nil {
		return s.current, errors.New("no previous config")
	}
	cur := s.current
	s.current = *s.previous
	s.previous = &cur
	if s.path != "" {
		_ = s.persist(s.current)
	}
	return s.current, nil
}

func (s *GameConfigStore) persist(cfg GameConfig) error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}

func (s *GameConfigStore) loadFromDisk() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var cfg GameConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return err
	}
	if err := ValidateGameConfig(cfg); err != nil {
		return err
	}
	s.mu.Lock()
	s.current = cfg
	s.mu.Unlock()
	return nil
}

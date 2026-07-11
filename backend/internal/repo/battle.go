package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"animalpoke/backend/internal/battle"
	"animalpoke/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrBattleNotFound    = errors.New("battle_not_found")
	ErrBattleNotOwner    = errors.New("battle_not_owner")
	ErrBattleAlreadyDone = errors.New("battle_already_settled")
	ErrBattleInvalidTeam = errors.New("battle_invalid_team")
	ErrBattleInvalidLog  = errors.New("battle_invalid_log")
	ErrBattleTamper      = errors.New("battle_command_tamper")
	ErrBattleUnknownArch = errors.New("battle_unknown_archetype")
)

// BattleRepo persists authoritative PvE sessions (AP-102).
type BattleRepo struct{ db *gorm.DB }

func NewBattleRepo(db *gorm.DB) *BattleRepo { return &BattleRepo{db: db} }

// StartRequest creates an open battle session.
type StartRequest struct {
	OwnerKey    string
	Mode        string
	ArchetypeID string
	Team        []battle.Fighter
}

// StartResponse is returned to the client before combat.
type StartResponse struct {
	Session   *models.BattleSession
	Team      []battle.Fighter
	Enemies   []battle.Fighter
	Threats   []battle.Threat
	CatalogRV string
}

// Start validates team, builds enemies from archetype, stores seed-bound snapshot.
func (r *BattleRepo) Start(req StartRequest) (*StartResponse, error) {
	owner := strings.TrimSpace(req.OwnerKey)
	if owner == "" {
		return nil, ErrInvalidOwner
	}
	archID := strings.TrimSpace(req.ArchetypeID)
	if archID == "" {
		archID = "bruiser"
	}
	if _, ok := battle.ArchetypeByID(archID); !ok {
		return nil, ErrBattleUnknownArch
	}
	team, err := battle.NormalizePlayerTeam(req.Team)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBattleInvalidTeam, err)
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "pve"
	}

	seedSrc := fmt.Sprintf("%s|%s|%s|%d", owner, archID, mode, time.Now().UTC().UnixNano())
	sum := sha256.Sum256([]byte(seedSrc))
	seed := hex.EncodeToString(sum[:16])

	enemies, err := battle.BuildEnemyTeam(archID, seed)
	if err != nil {
		return nil, err
	}
	threats := battle.AnalyzeThreats(team, archID)

	teamRaw, _ := json.Marshal(team)
	enemyRaw, _ := json.Marshal(enemies)
	threatRaw, _ := json.Marshal(threats)
	now := time.Now().UTC()
	sess := &models.BattleSession{
		SessionID:   uuid.NewString(),
		OwnerKey:    owner,
		Mode:        mode,
		ArchetypeID: archID,
		Seed:        seed,
		RuleVersion: battle.RuleVersion,
		Status:      "open",
		TeamJSON:    string(teamRaw),
		EnemyJSON:   string(enemyRaw),
		ThreatsJSON: string(threatRaw),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := r.db.Create(sess).Error; err != nil {
		return nil, err
	}
	return &StartResponse{
		Session:   sess,
		Team:      team,
		Enemies:   enemies,
		Threats:   threats,
		CatalogRV: battle.RuleVersion,
	}, nil
}

// SettleRequest carries client command log for authoritative replay.
type SettleRequest struct {
	OwnerKey  string
	SessionID string
	Commands  []battle.Command
	// Optional client-claimed winner — must match server if provided.
	ClaimedWinner string
}

// Settle replays seed+commands and completes the session once.
func (r *BattleRepo) Settle(req SettleRequest) (*models.BattleSession, *battle.Result, error) {
	var outSess *models.BattleSession
	var outRes *battle.Result
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var sess models.BattleSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("session_id = ?", req.SessionID).First(&sess).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBattleNotFound
			}
			return err
		}
		if sess.OwnerKey != req.OwnerKey {
			return ErrBattleNotOwner
		}
		if sess.Status == "completed" {
			var prev battle.Result
			_ = json.Unmarshal([]byte(sess.ResultJSON), &prev)
			outSess = &sess
			outRes = &prev
			return ErrBattleAlreadyDone
		}
		if sess.Status != "open" {
			return ErrBattleInvalidLog
		}

		var team []battle.Fighter
		var enemies []battle.Fighter
		if err := json.Unmarshal([]byte(sess.TeamJSON), &team); err != nil {
			return ErrBattleInvalidLog
		}
		if err := json.Unmarshal([]byte(sess.EnemyJSON), &enemies); err != nil {
			return ErrBattleInvalidLog
		}
		if len(req.Commands) > battle.MaxCommandsPerBattle {
			return ErrBattleInvalidLog
		}

		res, err := battle.Simulate(sess.Seed, team, enemies, req.Commands)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrBattleInvalidLog, err)
		}
		// reject client winner mismatch (tamper)
		if cw := strings.TrimSpace(req.ClaimedWinner); cw != "" && cw != res.WinnerSide {
			return ErrBattleTamper
		}

		cmdRaw, _ := json.Marshal(req.Commands)
		resRaw, _ := json.Marshal(res)
		now := time.Now().UTC()
		sess.Status = "completed"
		sess.CommandsJSON = string(cmdRaw)
		sess.ResultJSON = string(resRaw)
		sess.WinnerSide = res.WinnerSide
		sess.CommandHash = res.CommandHash
		sess.SettledAt = &now
		sess.UpdatedAt = now
		if err := tx.Save(&sess).Error; err != nil {
			return err
		}
		outSess = &sess
		outRes = &res
		return nil
	})
	if errors.Is(err, ErrBattleAlreadyDone) {
		return outSess, outRes, nil
	}
	return outSess, outRes, err
}

// Get returns a session by id.
func (r *BattleRepo) Get(sessionID string) (*models.BattleSession, error) {
	var sess models.BattleSession
	err := r.db.Where("session_id = ?", sessionID).First(&sess).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &sess, err
}

// VerifyCommandLog re-simulates an arbitrary snapshot (used by PvP extension).
func VerifyCommandLog(seed string, players, enemies []battle.Fighter, commands []battle.Command, claimed string) (*battle.Result, error) {
	res, err := battle.Simulate(seed, players, enemies, commands)
	if err != nil {
		return nil, err
	}
	if c := strings.TrimSpace(claimed); c != "" && c != res.WinnerSide && c != "player" && c != "enemy" && c != "draw" {
		// claimed may be owner keys in PvP; only compare when side-like
	}
	return &res, nil
}

package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"animalpoke/backend/internal/battle"
	"animalpoke/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPvPNotReady       = errors.New("pvp_not_ready")
	ErrPvPAlreadySettled = errors.New("pvp_already_settled")
	ErrPvPNotOwner       = errors.New("pvp_not_owner")
	ErrPvPInvalidLog     = errors.New("pvp_invalid_log")
)

// PvPRepo 匹配与段位结算（AP-115）。
type PvPRepo struct{ db *gorm.DB }

func NewPvPRepo(db *gorm.DB) *PvPRepo { return &PvPRepo{db: db} }

func (r *PvPRepo) getOrCreateRating(tx *gorm.DB, ownerKey string) (*models.PvPRating, error) {
	var row models.PvPRating
	err := tx.Where("owner_key = ?", ownerKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.PvPRating{OwnerKey: ownerKey, Rating: 1000, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err := tx.Create(&row).Error; err != nil {
			return nil, err
		}
		return &row, nil
	}
	return &row, err
}

// EnqueueOrMatch 入队；若有合适对手则创建 match。
func (r *PvPRepo) EnqueueOrMatch(ownerKey string) (match *models.PvPMatch, queued bool, err error) {
	ownerKey = strings.TrimSpace(ownerKey)
	if ownerKey == "" {
		return nil, false, ErrInvalidOwner
	}
	err = r.db.Transaction(func(tx *gorm.DB) error {
		me, err := r.getOrCreateRating(tx, ownerKey)
		if err != nil {
			return err
		}
		// 已在队列则返回排队
		var existing models.PvPQueue
		if err := tx.Where("owner_key = ?", ownerKey).First(&existing).Error; err == nil {
			queued = true
			return nil
		}
		// 找 rating 接近的对手（±200），FIFO
		var opp models.PvPQueue
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_key <> ? AND rating BETWEEN ? AND ?", ownerKey, me.Rating-200, me.Rating+200).
			Order("enqueued_at asc").First(&opp).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			q := models.PvPQueue{OwnerKey: ownerKey, Rating: me.Rating, EnqueuedAt: time.Now().UTC()}
			if err := tx.Create(&q).Error; err != nil {
				return err
			}
			queued = true
			return nil
		}
		if err != nil {
			return err
		}
		// 移除对手队列
		if err := tx.Delete(&opp).Error; err != nil {
			return err
		}
		seedSrc := fmt.Sprintf("%s|%s|%d", ownerKey, opp.OwnerKey, time.Now().UnixNano())
		sum := sha256.Sum256([]byte(seedSrc))
		m := &models.PvPMatch{
			MatchID: uuid.NewString(), PlayerA: opp.OwnerKey, PlayerB: ownerKey,
			Seed: hex.EncodeToString(sum[:8]), RuleVersion: "pvp.v1", Status: "matched",
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		match = m
		return nil
	})
	return match, queued, err
}

// CancelQueue 取消排队。
func (r *PvPRepo) CancelQueue(ownerKey string) error {
	return r.db.Where("owner_key = ?", ownerKey).Delete(&models.PvPQueue{}).Error
}

// SubmitResult 验证并原子结算（同一 match 只结算一次）。
// commandLog 必须包含 seed 与双方 id；winner 必须是 A 或 B。
func (r *PvPRepo) SubmitResult(ownerKey, matchID, winner string, commandLog map[string]any) (*models.PvPMatch, error) {
	var out *models.PvPMatch
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var m models.PvPMatch
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("match_id = ?", matchID).First(&m).Error; err != nil {
			return err
		}
		if m.PlayerA != ownerKey && m.PlayerB != ownerKey {
			return ErrPvPNotOwner
		}
		if m.Status == "completed" {
			out = &m
			return ErrPvPAlreadySettled
		}
		if m.Status != "matched" {
			return ErrPvPInvalidLog
		}
		// 校验 command log 基础字段
		if commandLog == nil {
			return ErrPvPInvalidLog
		}
		if seed, _ := commandLog["seed"].(string); seed != "" && seed != m.Seed {
			return ErrPvPInvalidLog
		}
		// AP-102: when full fighters+commands present, re-simulate and derive winner.
		if verified, ok, verr := verifyPvPBattleLog(m.Seed, commandLog); verr != nil {
			return ErrPvPInvalidLog
		} else if ok {
			derived := ""
			switch verified.WinnerSide {
			case "player", "a":
				derived = m.PlayerA
			case "enemy", "b":
				derived = m.PlayerB
			default:
				if verified.WinnerSide == m.PlayerA || verified.WinnerSide == m.PlayerB {
					derived = verified.WinnerSide
				}
			}
			if derived == "" {
				return ErrPvPInvalidLog
			}
			claimed := strings.TrimSpace(winner)
			if claimed != "" && claimed != derived {
				return ErrPvPInvalidLog
			}
			winner = derived
			commandLog["server_result"] = map[string]any{
				"winner_side":  verified.WinnerSide,
				"command_hash": verified.CommandHash,
				"rounds":       verified.Rounds,
				"rule_version": verified.RuleVersion,
			}
		}
		winner = strings.TrimSpace(winner)
		if winner != m.PlayerA && winner != m.PlayerB {
			return ErrPvPInvalidLog
		}
		// ELO
		ra, err := r.getOrCreateRating(tx, m.PlayerA)
		if err != nil {
			return err
		}
		rb, err := r.getOrCreateRating(tx, m.PlayerB)
		if err != nil {
			return err
		}
		scoreA, scoreB := 0.0, 1.0
		if winner == m.PlayerA {
			scoreA, scoreB = 1.0, 0.0
			ra.Wins++
			rb.Losses++
		} else {
			rb.Wins++
			ra.Losses++
		}
		const k = 32.0
		ea := 1.0 / (1.0 + math.Pow(10, float64(rb.Rating-ra.Rating)/400.0))
		eb := 1.0 - ea
		ra.Rating = int(math.Round(float64(ra.Rating) + k*(scoreA-ea)))
		rb.Rating = int(math.Round(float64(rb.Rating) + k*(scoreB-eb)))
		ra.UpdatedAt = time.Now().UTC()
		rb.UpdatedAt = time.Now().UTC()
		if err := tx.Save(ra).Error; err != nil {
			return err
		}
		if err := tx.Save(rb).Error; err != nil {
			return err
		}
		raw, _ := json.Marshal(commandLog)
		now := time.Now().UTC()
		m.Status = "completed"
		m.Winner = winner
		m.ResultJSON = string(raw)
		m.SettledAt = &now
		m.UpdatedAt = now
		if err := tx.Save(&m).Error; err != nil {
			return err
		}
		out = &m
		return nil
	})
	if errors.Is(err, ErrPvPAlreadySettled) {
		return out, nil // 幂等返回已结算
	}
	return out, err
}

// GetMatch 查询。
func (r *PvPRepo) GetMatch(matchID string) (*models.PvPMatch, error) {
	var m models.PvPMatch
	err := r.db.Where("match_id = ?", matchID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &m, err
}

// GetRating 查询段位。
func (r *PvPRepo) GetRating(ownerKey string) (*models.PvPRating, error) {
	return r.getOrCreateRating(r.db, ownerKey)
}

// verifyPvPBattleLog replays AP-102 shaped logs: {players|team_a, enemies|team_b, commands}.
// ok=false means legacy minimal log (winner trusted after basic checks).
func verifyPvPBattleLog(seed string, commandLog map[string]any) (*battle.Result, bool, error) {
	if commandLog == nil {
		return nil, false, nil
	}
	rawPlayers, hasP := commandLog["players"]
	if !hasP {
		rawPlayers, hasP = commandLog["team_a"]
	}
	rawEnemies, hasE := commandLog["enemies"]
	if !hasE {
		rawEnemies, hasE = commandLog["team_b"]
	}
	rawCmds, hasC := commandLog["commands"]
	if !hasP || !hasE || !hasC {
		return nil, false, nil
	}
	pj, _ := json.Marshal(rawPlayers)
	ej, _ := json.Marshal(rawEnemies)
	cj, _ := json.Marshal(rawCmds)
	var players []battle.Fighter
	var enemies []battle.Fighter
	var commands []battle.Command
	if err := json.Unmarshal(pj, &players); err != nil {
		return nil, true, err
	}
	if err := json.Unmarshal(ej, &enemies); err != nil {
		return nil, true, err
	}
	if err := json.Unmarshal(cj, &commands); err != nil {
		return nil, true, err
	}
	if len(players) == 0 || len(enemies) == 0 {
		return nil, true, fmt.Errorf("empty teams")
	}
	for i := range players {
		players[i].Side = "player"
		if players[i].MaxHP <= 0 {
			players[i].MaxHP = players[i].HP
		}
	}
	for i := range enemies {
		enemies[i].Side = "enemy"
		if enemies[i].MaxHP <= 0 {
			enemies[i].MaxHP = enemies[i].HP
		}
	}
	useSeed := seed
	if s, _ := commandLog["seed"].(string); s != "" {
		useSeed = s
	}
	res, err := battle.Simulate(useSeed, players, enemies, commands)
	if err != nil {
		return nil, true, err
	}
	return &res, true, nil
}

package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RankingRepo 区域日榜与结算（AP-114）。
type RankingRepo struct {
	db *gorm.DB
}

func NewRankingRepo(db *gorm.DB) *RankingRepo { return &RankingRepo{db: db} }

// RankingOwner 榜单主体。
type RankingOwner struct {
	Type string // account|device
	ID   string
}

// AddScore 累加当日分数（eligible=false 不计榜）。
func (r *RankingRepo) AddScore(date, city string, owner RankingOwner, delta int64, display string, eligible bool) error {
	if delta == 0 {
		return nil
	}
	city = normalizeCity(city)
	date = normalizeDate(date)
	return r.db.Transaction(func(tx *gorm.DB) error {
		var row models.RankingDailyScore
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("date = ? AND city = ? AND owner_type = ? AND owner_id = ?", date, city, owner.Type, owner.ID).
			First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			score := delta
			if !eligible || score < 0 {
				score = 0
			}
			now := time.Now().UTC()
			return tx.Exec(
				`INSERT INTO ranking_daily_scores (date, city, owner_type, owner_id, score, eligible, display, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
				date, city, owner.Type, owner.ID, score, eligible, display, now, now,
			).Error
		}
		if err != nil {
			return err
		}
		if !eligible {
			row.Eligible = false
		} else if row.Eligible {
			row.Score += delta
			if row.Score < 0 {
				row.Score = 0
			}
		}
		if display != "" {
			row.Display = display
		}
		row.UpdatedAt = time.Now().UTC()
		return tx.Save(&row).Error
	})
}

// ListBoard 分页排行（仅 eligible）。
func (r *RankingRepo) ListBoard(date, city string, offset, limit int) ([]models.RankingDailyScore, int64, error) {
	city = normalizeCity(city)
	date = normalizeDate(date)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := r.db.Model(&models.RankingDailyScore{}).
		Where("date = ? AND city = ? AND eligible = ?", date, city, true)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.RankingDailyScore
	err := q.Order("score desc, updated_at asc").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}

// MyRank 我的排名（1-based；0 表示未上榜）。
func (r *RankingRepo) MyRank(date, city string, owner RankingOwner) (rank int, score int64, err error) {
	city = normalizeCity(city)
	date = normalizeDate(date)
	var me models.RankingDailyScore
	err = r.db.Where("date = ? AND city = ? AND owner_type = ? AND owner_id = ? AND eligible = ?",
		date, city, owner.Type, owner.ID, true).First(&me).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	var better int64
	if err := r.db.Model(&models.RankingDailyScore{}).
		Where("date = ? AND city = ? AND eligible = ? AND (score > ? OR (score = ? AND updated_at < ?))",
			date, city, true, me.Score, me.Score, me.UpdatedAt).
		Count(&better).Error; err != nil {
		return 0, 0, err
	}
	return int(better) + 1, me.Score, nil
}

// GetSnapshot 取结算快照。
func (r *RankingRepo) GetSnapshot(date, city string) (*models.RankingSnapshot, error) {
	var s models.RankingSnapshot
	err := r.db.Where("date = ? AND city = ?", normalizeDate(date), normalizeCity(city)).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &s, err
}

// SettleCity 生成不可变快照；已存在则返回原快照。
func (r *RankingRepo) SettleCity(date, city string) (*models.RankingSnapshot, error) {
	city = normalizeCity(city)
	date = normalizeDate(date)
	if existing, err := r.GetSnapshot(date, city); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	rows, _, err := r.ListBoard(date, city, 0, 100)
	if err != nil {
		return nil, err
	}
	type entry struct {
		Rank    int    `json:"rank"`
		Owner   string `json:"owner_id"`
		Type    string `json:"owner_type"`
		Score   int64  `json:"score"`
		Display string `json:"display_name"`
	}
	entries := make([]entry, 0, len(rows))
	for i, row := range rows {
		entries = append(entries, entry{
			Rank: i + 1, Owner: row.OwnerID, Type: row.OwnerType,
			Score: row.Score, Display: row.Display,
		})
	}
	raw, _ := json.Marshal(entries)
	snap := &models.RankingSnapshot{
		SnapshotID:  uuid.NewString(),
		Date:        date,
		City:        city,
		EntriesJSON: string(raw),
		SettledAt:   time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
	}
	if err := r.db.Create(snap).Error; err != nil {
		if existing, e2 := r.GetSnapshot(date, city); e2 == nil && existing != nil {
			return existing, nil
		}
		return nil, err
	}
	return snap, nil
}

// GrantTopRewards 对快照 TopN 发金币（wallet），幂等。
func (r *RankingRepo) GrantTopRewards(snap *models.RankingSnapshot, wallet *WalletRepo, topN int, goldForRank func(rank int) int64) error {
	if snap == nil || wallet == nil {
		return fmt.Errorf("nil deps")
	}
	if topN <= 0 {
		topN = 3
	}
	var entries []struct {
		Rank    int    `json:"rank"`
		Owner   string `json:"owner_id"`
		Type    string `json:"owner_type"`
		Score   int64  `json:"score"`
		Display string `json:"display_name"`
	}
	if err := json.Unmarshal([]byte(snap.EntriesJSON), &entries); err != nil {
		return err
	}
	for _, e := range entries {
		if e.Rank > topN {
			break
		}
		gold := goldForRank(e.Rank)
		if gold <= 0 {
			continue
		}
		opID := fmt.Sprintf("rank:%s:%s:%d", snap.SnapshotID, e.Owner, e.Rank)
		gr := models.RankingRewardGrant{
			SnapshotID: snap.SnapshotID, OwnerType: e.Type, OwnerID: e.Owner,
			Rank: e.Rank, Gold: gold, OperationID: opID, CreatedAt: time.Now().UTC(),
		}
		if err := r.db.Create(&gr).Error; err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") {
				continue
			}
			return err
		}
		accountID, deviceID := "", e.Owner
		if e.Type == "account" {
			accountID, deviceID = e.Owner, "rank-system"
		}
		_, err := wallet.Apply(ApplyRequest{
			DeviceID: deviceID, AccountID: accountID,
			Kind: models.LedgerKindCurrency, Currency: models.CurrencyGold,
			Delta: gold, OperationID: opID, SourceType: "ranking_reward", SourceID: snap.SnapshotID,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func normalizeCity(city string) string {
	c := strings.TrimSpace(strings.ToLower(city))
	if c == "" {
		return "unknown"
	}
	if len(c) > 64 {
		c = c[:64]
	}
	return c
}

func normalizeDate(date string) string {
	date = strings.TrimSpace(date)
	if date == "" {
		return time.Now().UTC().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return time.Now().UTC().Format("2006-01-02")
	}
	return date
}

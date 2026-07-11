// Package repo — AP-098 photography quality persistence.
package repo

import (
	"errors"
	"strings"
	"time"

	"animalpoke/backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Photo quality domain errors.
var (
	ErrPhotoRepoUnavailable = errors.New("photo repo unavailable")
	ErrPhotoDailyCap        = errors.New("photo_daily_cap")
	ErrPhotoDuplicate       = errors.New("photo_duplicate_metrics")
)

// PhotoScoreDailyCap mirrors services.PhotoScoreDailyCap for repo checks.
const PhotoScoreDailyCap = 40

// PhotoRepo stores calibration, scores, personal bests and theme progress.
type PhotoRepo struct {
	db *gorm.DB
}

// NewPhotoRepo constructs a repo.
func NewPhotoRepo(db *gorm.DB) *PhotoRepo {
	return &PhotoRepo{db: db}
}

// GetCalibrationRow loads owner calibration row (nil if none).
func (r *PhotoRepo) GetCalibrationRow(ownerKey string) (*models.PhotoDeviceCalibration, error) {
	if r == nil || r.db == nil {
		return nil, ErrPhotoRepoUnavailable
	}
	var row models.PhotoDeviceCalibration
	err := r.db.Where("owner_key = ?", ownerKey).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// UpsertCalibrationRow writes a full calibration snapshot (handler blends samples).
func (r *PhotoRepo) UpsertCalibrationRow(row *models.PhotoDeviceCalibration) (*models.PhotoDeviceCalibration, error) {
	if r == nil || r.db == nil || row == nil {
		return nil, ErrPhotoRepoUnavailable
	}
	var out models.PhotoDeviceCalibration
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing models.PhotoDeviceCalibration
		err := tx.Where("owner_key = ?", row.OwnerKey).First(&existing).Error
		now := time.Now().UTC()
		if err == gorm.ErrRecordNotFound {
			row.CreatedAt = now
			row.UpdatedAt = now
			if err := tx.Create(row).Error; err != nil {
				return err
			}
			out = *row
			return nil
		}
		if err != nil {
			return err
		}
		existing.BaselineStabilityRMS = row.BaselineStabilityRMS
		existing.LightingOffset = row.LightingOffset
		existing.SampleCount = row.SampleCount
		existing.Calibrated = row.Calibrated
		existing.ConfigVersion = row.ConfigVersion
		if row.DeviceModel != "" {
			existing.DeviceModel = row.DeviceModel
		}
		if row.DeviceID != "" {
			existing.DeviceID = row.DeviceID
		}
		if row.AccountID != "" {
			existing.AccountID = row.AccountID
		}
		existing.UpdatedAt = now
		if err := tx.Save(&existing).Error; err != nil {
			return err
		}
		out = existing
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// CountScoresToday returns how many scores the owner submitted today (UTC).
func (r *PhotoRepo) CountScoresToday(ownerKey, dayKey string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, ErrPhotoRepoUnavailable
	}
	var n int64
	err := r.db.Model(&models.PhotoScoreRecord{}).
		Where("owner_key = ? AND day_key = ?", ownerKey, dayKey).
		Count(&n).Error
	return n, err
}

// SaveScoreInput is a primitive payload (avoids services import cycle).
type SaveScoreInput struct {
	OwnerKey       string
	DeviceID       string
	AccountID      string
	Overall        float64
	Band           string
	Stability      float64
	Completeness   float64
	Lighting       float64
	Occlusion      float64
	Composition    float64
	SafeDistance   float64
	ChasePenalty   bool
	RarityEligible bool
	MetricsDigest  string
	Signature      string
	ConfigVersion  string
	ThemeID        string
	ThemeDimScore  float64
	ThemeMet       bool
	A11yCompleted  bool
	DailyCap       int
}

// SaveScoreResult persists a score, updates personal best and optional theme progress.
func (r *PhotoRepo) SaveScoreResult(in SaveScoreInput) (*models.PhotoScoreRecord, *models.PhotoPersonalBest, error) {
	if r == nil || r.db == nil {
		return nil, nil, ErrPhotoRepoUnavailable
	}
	dayKey := time.Now().UTC().Format("2006-01-02")
	cap := in.DailyCap
	if cap <= 0 {
		cap = PhotoScoreDailyCap
	}
	var record models.PhotoScoreRecord
	var best models.PhotoPersonalBest

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&models.PhotoScoreRecord{}).
			Where("owner_key = ? AND day_key = ?", in.OwnerKey, dayKey).
			Count(&n).Error; err != nil {
			return err
		}
		if n >= int64(cap) {
			return ErrPhotoDailyCap
		}
		var dup int64
		if err := tx.Model(&models.PhotoScoreRecord{}).
			Where("owner_key = ? AND day_key = ? AND metrics_digest = ?", in.OwnerKey, dayKey, in.MetricsDigest).
			Count(&dup).Error; err != nil {
			return err
		}
		if dup > 0 {
			return ErrPhotoDuplicate
		}

		now := time.Now().UTC()
		record = models.PhotoScoreRecord{
			ScoreID:        uuid.NewString(),
			OwnerKey:       in.OwnerKey,
			DeviceID:       in.DeviceID,
			AccountID:      in.AccountID,
			DayKey:         dayKey,
			Overall:        in.Overall,
			Band:           in.Band,
			Stability:      in.Stability,
			Completeness:   in.Completeness,
			Lighting:       in.Lighting,
			Occlusion:      in.Occlusion,
			Composition:    in.Composition,
			SafeDistance:   in.SafeDistance,
			ChasePenalty:   in.ChasePenalty,
			RarityEligible: in.RarityEligible,
			MetricsDigest:  in.MetricsDigest,
			Signature:      in.Signature,
			ConfigVersion:  in.ConfigVersion,
			ThemeID:        in.ThemeID,
			A11yCompleted:  in.A11yCompleted,
			CreatedAt:      now,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}

		err := tx.Where("owner_key = ?", in.OwnerKey).First(&best).Error
		if err == gorm.ErrRecordNotFound {
			best = models.PhotoPersonalBest{
				OwnerKey:     in.OwnerKey,
				DeviceID:     in.DeviceID,
				AccountID:    in.AccountID,
				Overall:      in.Overall,
				Stability:    in.Stability,
				Completeness: in.Completeness,
				Lighting:     in.Lighting,
				Occlusion:    in.Occlusion,
				Composition:  in.Composition,
				SafeDistance: in.SafeDistance,
				BestScoreID:  record.ScoreID,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err := tx.Create(&best).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			updated := false
			if in.Overall > best.Overall {
				best.Overall = in.Overall
				best.BestScoreID = record.ScoreID
				updated = true
			}
			if in.Stability > best.Stability {
				best.Stability = in.Stability
				updated = true
			}
			if in.Completeness > best.Completeness {
				best.Completeness = in.Completeness
				updated = true
			}
			if in.Lighting > best.Lighting {
				best.Lighting = in.Lighting
				updated = true
			}
			if in.Occlusion > best.Occlusion {
				best.Occlusion = in.Occlusion
				updated = true
			}
			if in.Composition > best.Composition {
				best.Composition = in.Composition
				updated = true
			}
			if in.SafeDistance > best.SafeDistance {
				best.SafeDistance = in.SafeDistance
				updated = true
			}
			if updated {
				best.DeviceID = in.DeviceID
				if in.AccountID != "" {
					best.AccountID = in.AccountID
				}
				best.UpdatedAt = now
				if err := tx.Save(&best).Error; err != nil {
					return err
				}
			}
		}

		if in.ThemeID != "" {
			var prog models.PhotoThemeProgress
			perr := tx.Where("owner_key = ? AND day_key = ?", in.OwnerKey, dayKey).First(&prog).Error
			if perr == gorm.ErrRecordNotFound {
				prog = models.PhotoThemeProgress{
					OwnerKey:      in.OwnerKey,
					DayKey:        dayKey,
					ThemeID:       in.ThemeID,
					DeviceID:      in.DeviceID,
					AccountID:     in.AccountID,
					Completed:     in.ThemeMet,
					A11yCompleted: in.A11yCompleted,
					BestDimScore:  in.ThemeDimScore,
					CreatedAt:     now,
					UpdatedAt:     now,
				}
				if in.ThemeMet {
					prog.CompletedAt = &now
				}
				if err := tx.Create(&prog).Error; err != nil {
					return err
				}
			} else if perr != nil {
				return perr
			} else {
				if in.ThemeDimScore > prog.BestDimScore {
					prog.BestDimScore = in.ThemeDimScore
				}
				if in.A11yCompleted {
					prog.A11yCompleted = true
				}
				if in.ThemeMet && !prog.Completed {
					prog.Completed = true
					prog.CompletedAt = &now
				}
				prog.UpdatedAt = now
				if err := tx.Save(&prog).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &record, &best, nil
}

// GetPersonalBest loads best scores.
func (r *PhotoRepo) GetPersonalBest(ownerKey string) (*models.PhotoPersonalBest, error) {
	if r == nil || r.db == nil {
		return nil, ErrPhotoRepoUnavailable
	}
	var best models.PhotoPersonalBest
	err := r.db.Where("owner_key = ?", ownerKey).First(&best).Error
	if err == gorm.ErrRecordNotFound {
		return &models.PhotoPersonalBest{OwnerKey: ownerKey}, nil
	}
	if err != nil {
		return nil, err
	}
	return &best, nil
}

// GetThemeProgress loads today's theme progress.
func (r *PhotoRepo) GetThemeProgress(ownerKey, dayKey string) (*models.PhotoThemeProgress, error) {
	if r == nil || r.db == nil {
		return nil, ErrPhotoRepoUnavailable
	}
	var prog models.PhotoThemeProgress
	err := r.db.Where("owner_key = ? AND day_key = ?", ownerKey, dayKey).First(&prog).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &prog, nil
}

// MarkA11yThemeComplete marks accessibility alternative complete for the day.
func (r *PhotoRepo) MarkA11yThemeComplete(ownerKey, deviceID, accountID, themeID string) (*models.PhotoThemeProgress, error) {
	if r == nil || r.db == nil {
		return nil, ErrPhotoRepoUnavailable
	}
	now := time.Now().UTC()
	dayKey := now.Format("2006-01-02")
	var prog models.PhotoThemeProgress
	err := r.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("owner_key = ? AND day_key = ?", ownerKey, dayKey).First(&prog).Error
		if err == gorm.ErrRecordNotFound {
			prog = models.PhotoThemeProgress{
				OwnerKey:      ownerKey,
				DayKey:        dayKey,
				ThemeID:       themeID,
				DeviceID:      deviceID,
				AccountID:     accountID,
				Completed:     true,
				A11yCompleted: true,
				CompletedAt:   &now,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			return tx.Create(&prog).Error
		}
		if err != nil {
			return err
		}
		prog.A11yCompleted = true
		prog.Completed = true
		if prog.CompletedAt == nil {
			prog.CompletedAt = &now
		}
		prog.UpdatedAt = now
		return tx.Save(&prog).Error
	})
	if err != nil {
		return nil, err
	}
	return &prog, nil
}

// EnsureOwnerKey validates owner key non-empty.
func EnsurePhotoOwnerKey(accountID, deviceID string) (string, error) {
	ok := OwnerKey(accountID, deviceID)
	if strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(ok, "acc:"), "dev:")) == "" {
		return "", ErrInvalidOwner
	}
	return ok, nil
}

package repo

import (
	"errors"
	"strings"
	"time"

	"animalpoke/backend/internal/models"

	"gorm.io/gorm"
)

var (
	ErrAdventureNotFound        = errors.New("adventure_not_found")
	ErrAdventureChoiceConflict  = errors.New("adventure_choice_conflict")
	ErrAdventureRepoUnavailable = errors.New("adventure_repo_unavailable")
)

type AdventureRepo struct {
	db *gorm.DB
}

func NewAdventureRepo(db *gorm.DB) *AdventureRepo {
	return &AdventureRepo{db: db}
}

func (r *AdventureRepo) WithTx(tx *gorm.DB) *AdventureRepo {
	return &AdventureRepo{db: tx}
}

// Transaction lets adventure state and related domain writes commit together.
func (r *AdventureRepo) Transaction(fn func(*gorm.DB) error) error {
	if r == nil || r.db == nil {
		return ErrAdventureRepoUnavailable
	}
	return r.db.Transaction(fn)
}

func (r *AdventureRepo) Create(run *models.AdventureRun) error {
	if r == nil || r.db == nil {
		return ErrAdventureRepoUnavailable
	}
	return r.db.Create(run).Error
}

func (r *AdventureRepo) FindOwned(runID, accountID, deviceID string) (*models.AdventureRun, error) {
	if r == nil || r.db == nil {
		return nil, ErrAdventureRepoUnavailable
	}
	var run models.AdventureRun
	q := r.db.Where("run_id = ?", strings.TrimSpace(runID))
	if accountID != "" {
		q = q.Where("(device_id = ? OR account_id = ?)", deviceID, accountID)
	} else {
		q = q.Where("device_id = ?", deviceID)
	}
	err := q.First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAdventureNotFound
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *AdventureRepo) FindByOperation(operationID, accountID, deviceID string) (*models.AdventureRun, error) {
	if r == nil || r.db == nil {
		return nil, ErrAdventureRepoUnavailable
	}
	q := r.db.Where("operation_id = ?", strings.TrimSpace(operationID))
	if accountID != "" {
		q = q.Where("(device_id = ? OR account_id = ?)", deviceID, accountID)
	} else {
		q = q.Where("device_id = ?", deviceID)
	}
	var run models.AdventureRun
	err := q.First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAdventureNotFound
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *AdventureRepo) Complete(runID, accountID, deviceID, choiceID, outcome, souvenir string, bondDelta int) (*models.AdventureRun, bool, error) {
	run, err := r.FindOwned(runID, accountID, deviceID)
	if err != nil {
		return nil, false, err
	}
	if run.Status == "completed" {
		if run.SelectedChoiceID != choiceID {
			return run, true, ErrAdventureChoiceConflict
		}
		return run, true, nil
	}
	now := time.Now().UTC()
	q := r.db.Model(&models.AdventureRun{}).Where("run_id = ? AND status = ?", runID, "generated")
	if accountID != "" {
		q = q.Where("(device_id = ? OR account_id = ?)", deviceID, accountID)
	} else {
		q = q.Where("device_id = ?", deviceID)
	}
	res := q.
		Updates(map[string]interface{}{
			"status":             "completed",
			"selected_choice_id": choiceID,
			"outcome":            outcome,
			"souvenir_name":      souvenir,
			"bond_delta":         bondDelta,
			"completed_at":       now,
		})
	if res.Error != nil {
		return nil, false, res.Error
	}
	if res.RowsAffected == 0 {
		fresh, findErr := r.FindOwned(runID, accountID, deviceID)
		if findErr != nil {
			return nil, false, findErr
		}
		if fresh.Status == "completed" && fresh.SelectedChoiceID == choiceID {
			return fresh, true, nil
		}
		return fresh, true, ErrAdventureChoiceConflict
	}
	completed, err := r.FindOwned(runID, accountID, deviceID)
	return completed, false, err
}

func (r *AdventureRepo) ListOwned(accountID, deviceID, animalUUID string, limit int) ([]models.AdventureRun, error) {
	if r == nil || r.db == nil {
		return nil, ErrAdventureRepoUnavailable
	}
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	q := r.db.Model(&models.AdventureRun{})
	if accountID != "" {
		q = q.Where("(device_id = ? OR account_id = ?)", deviceID, accountID)
	} else {
		q = q.Where("device_id = ?", deviceID)
	}
	if strings.TrimSpace(animalUUID) != "" {
		q = q.Where("animal_uuid = ?", strings.TrimSpace(animalUUID))
	}
	var rows []models.AdventureRun
	err := q.Order("created_at desc").Limit(limit).Find(&rows).Error
	return rows, err
}

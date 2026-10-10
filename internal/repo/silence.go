package repo

import (
	"time"
	"watchAlert/internal/models"

	"gorm.io/gorm"
)

type (
	SilenceRepo struct {
		entryRepo
	}

	InterSilenceRepo interface {
		List(tenantId, faultCenterId, query string, status string, page models.Page) ([]models.AlertSilences, int64, error)
		Create(r models.AlertSilences) error
		Update(r models.AlertSilences) error
		Delete(tenantId, id string) error
	}
)

func newSilenceInterface(db *gorm.DB, g InterGormDBCli) InterSilenceRepo {
	return &SilenceRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

func (sr SilenceRepo) List(tenantId, faultCenterId, query string, status string, page models.Page) ([]models.AlertSilences, int64, error) {
	var (
		silenceList []models.AlertSilences
		count       int64
	)
	db := sr.db.Model(models.AlertSilences{})
	if tenantId != "" {
		db.Where("tenant_id = ?", tenantId)
	}

	if faultCenterId != "" {
		db.Where("fault_center_id = ?", faultCenterId)
	}

	if query != "" {
		db.Where("(id LIKE ? OR name LIKE ? OR comment LIKE ?)", "%"+query+"%", "%"+query+"%", "%"+query+"%")
	}

	// Derive status from the window, avoiding background writes that can
	// overwrite a user's reschedule or recreate a deleted cache entry.
	now := time.Now().Unix()
	switch status {
	case "0":
		db = db.Where("starts_at > ? AND ends_at > starts_at", now)
	case "1":
		db = db.Where("starts_at <= ? AND ends_at > ? AND ends_at > starts_at", now, now)
	case "2":
		db = db.Where("ends_at <= ? OR ends_at <= starts_at", now)
	case "", "all":
	default:
		db = db.Where("status = ?", status)
	}

	db.Count(&count)
	db.Order("update_at DESC, id DESC").Limit(int(page.Size)).Offset(int((page.Index - 1) * page.Size))
	err := db.Find(&silenceList).Error
	if err != nil {
		return nil, 0, err
	}

	for i := range silenceList {
		silenceList[i].Status = silenceList[i].StatusAt(now)
	}
	return silenceList, count, nil
}

func (sr SilenceRepo) Create(r models.AlertSilences) error {
	err := sr.g.Create(models.AlertSilences{}, r)
	if err != nil {
		return err
	}

	return nil
}

func (sr SilenceRepo) Update(r models.AlertSilences) error {
	return sr.db.Transaction(func(tx *gorm.DB) error {
		var existing models.AlertSilences
		if err := tx.Where("tenant_id = ? AND id = ?", r.TenantId, r.ID).First(&existing).Error; err != nil {
			return err
		}
		// Include zero values so rescheduling an active silence persists status=0.
		return tx.Model(&models.AlertSilences{}).
			Where("tenant_id = ? AND id = ?", r.TenantId, r.ID).
			Select("name", "labels", "starts_at", "ends_at", "update_at", "update_by", "fault_center_id", "comment", "status").
			Updates(r).Error
	})
}

func (sr SilenceRepo) Delete(tenantId, id string) error {
	var silence models.AlertSilences
	db := sr.db.Where("tenant_id = ? AND id = ?", tenantId, id)
	err := db.First(&silence).Error
	if err != nil {
		return err
	}

	del := Delete{
		Table: models.AlertSilences{},
		Where: map[string]interface{}{
			"tenant_id = ?": tenantId,
			"id = ?":        id,
		},
	}
	err = sr.g.Delete(del)
	if err != nil {
		return err
	}

	return nil
}

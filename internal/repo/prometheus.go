package repo

import (
	"fmt"
	"watchAlert/internal/models"

	"gorm.io/gorm"
)

// -----------------------------------------------------------------------
// TargetGroup
// -----------------------------------------------------------------------

type (
	PrometheusTargetGroupRepo struct {
		entryRepo
	}

	InterPrometheusTargetGroupRepo interface {
		List(tenantId, query string, page models.Page) ([]models.PrometheusTargetGroup, int64, error)
		ListAllByTenant(tenantId string) ([]models.PrometheusTargetGroup, error)
		Create(req *models.PrometheusTargetGroup) error
		Update(req *models.PrometheusTargetGroup) error
		Delete(tenantId string, id int64) error
		Get(tenantId string, id int64) (models.PrometheusTargetGroup, error)
	}
)

func newPrometheusTargetGroupInterface(db *gorm.DB, g InterGormDBCli) InterPrometheusTargetGroupRepo {
	return &PrometheusTargetGroupRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

func (r PrometheusTargetGroupRepo) List(tenantId, query string, page models.Page) ([]models.PrometheusTargetGroup, int64, error) {
	var (
		data  []models.PrometheusTargetGroup
		db    = r.db.Model(&models.PrometheusTargetGroup{})
		count int64
	)

	db.Where("tenant_id = ?", tenantId)

	if query != "" {
		db.Where("name LIKE ?", "%"+query+"%")
	}

	db.Count(&count)
	db.Limit(int(page.Size)).Offset(int((page.Index - 1) * page.Size))

	err := db.Find(&data).Error
	if err != nil {
		return nil, 0, err
	}

	return data, count, nil
}

func (r PrometheusTargetGroupRepo) ListAllByTenant(tenantId string) ([]models.PrometheusTargetGroup, error) {
	var data []models.PrometheusTargetGroup
	err := r.db.Model(&models.PrometheusTargetGroup{}).
		Where("tenant_id = ?", tenantId).
		Find(&data).Error
	return data, err
}

func (r PrometheusTargetGroupRepo) Create(req *models.PrometheusTargetGroup) error {
	var existing models.PrometheusTargetGroup
	r.db.Model(&models.PrometheusTargetGroup{}).
		Where("tenant_id = ? AND name = ?", req.TenantId, req.Name).
		First(&existing)
	if existing.ID > 0 {
		return fmt.Errorf("服务组名称已存在")
	}

	return r.g.Create(&models.PrometheusTargetGroup{}, req)
}

func (r PrometheusTargetGroupRepo) Update(req *models.PrometheusTargetGroup) error {
	u := Updates{
		Table: &models.PrometheusTargetGroup{},
		Where: map[string]interface{}{
			"tenant_id = ?": req.TenantId,
			"id = ?":        req.ID,
		},
		Updates: req,
	}
	return r.g.Updates(u)
}

func (r PrometheusTargetGroupRepo) Delete(tenantId string, id int64) error {
	var targetCount int64
	r.db.Model(&models.PrometheusTargetGroup{}).
		Where("tenant_id = ? AND target_group_id = ?", tenantId, id).
		Count(&targetCount)
	if targetCount != 0 {
		return fmt.Errorf("禁止删除, 目标组 %d 不为空", id)	
	}

	d := Delete{
		Table: models.PrometheusTargetGroup{},
		Where: map[string]interface{}{
			"tenant_id = ?": tenantId,
			"id = ?":        id,
		},
	}
	return r.g.Delete(d)
}

func (r PrometheusTargetGroupRepo) Get(tenantId string, id int64) (models.PrometheusTargetGroup, error) {
	var data models.PrometheusTargetGroup
	err := r.db.Model(&models.PrometheusTargetGroup{}).
		Where("tenant_id = ? AND id = ?", tenantId, id).
		First(&data).Error
	return data, err
}

// -----------------------------------------------------------------------
// Target
// -----------------------------------------------------------------------

type (
	PrometheusTargetRepo struct {
		entryRepo
	}

	InterPrometheusTargetRepo interface {
		List(tenantId string, groupId int64, query string, page models.Page) ([]models.PrometheusTarget, int64, error)
		ListAllByTenant(tenantId string) ([]models.PrometheusTarget, error)
		Create(req *models.PrometheusTarget) error
		Update(req *models.PrometheusTarget) error
		Delete(tenantId string, groupId, id int64) error
		Get(tenantId string, groupId, id int64) (models.PrometheusTarget, error)
	}
)

func newPrometheusTargetInterface(db *gorm.DB, g InterGormDBCli) InterPrometheusTargetRepo {
	return &PrometheusTargetRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

func (r PrometheusTargetRepo) List(tenantId string, groupId int64, query string, page models.Page) ([]models.PrometheusTarget, int64, error) {
	var (
		data  []models.PrometheusTarget
		db    = r.db.Model(&models.PrometheusTarget{})
		count int64
	)

	db.Where("tenant_id = ?", tenantId)

	if groupId != 0 {
		db.Where("group_id = ?", groupId)
	}

	if query != "" {
		db.Where("targets LIKE ?", "%"+query+"%")
	}

	db.Count(&count)
	db.Limit(int(page.Size)).Offset(int((page.Index - 1) * page.Size))

	err := db.Find(&data).Error
	if err != nil {
		return nil, 0, err
	}

	return data, count, nil
}

func (r PrometheusTargetRepo) ListAllByTenant(tenantId string) ([]models.PrometheusTarget, error) {
	var data []models.PrometheusTarget
	err := r.db.Model(&models.PrometheusTarget{}).
		Where("tenant_id = ?", tenantId).
		Find(&data).Error
	return data, err
}

func (r PrometheusTargetRepo) Create(req *models.PrometheusTarget) error {
	return r.g.Create(&models.PrometheusTarget{}, req)
}

func (r PrometheusTargetRepo) Update(req *models.PrometheusTarget) error {
	u := Updates{
		Table: &models.PrometheusTarget{},
		Where: map[string]interface{}{
			"tenant_id = ?": req.TenantId,
			"id = ?":        req.ID,
		},
		Updates: req,
	}
	return r.g.Updates(u)
}

func (r PrometheusTargetRepo) Delete(tenantId string, groupId, id int64) error {
	d := Delete{
		Table: models.PrometheusTarget{},
		Where: map[string]interface{}{
			"tenant_id = ?": tenantId,
			"group_id = ?":  groupId,
			"id = ?":        id,
		},
	}
	return r.g.Delete(d)
}

func (r PrometheusTargetRepo) Get(tenantId string, groupId, id int64) (models.PrometheusTarget, error) {
	var data models.PrometheusTarget
	q := r.db.Model(&models.PrometheusTarget{}).Where("tenant_id = ?", tenantId)
	if groupId != 0 {
		q = q.Where("group_id = ?", groupId)
	}
	err := q.Where("id = ?", id).First(&data).Error
	return data, err
}

// -----------------------------------------------------------------------
// TargetVersion
// -----------------------------------------------------------------------

type (
	PrometheusTargetVersionRepo struct {
		entryRepo
	}

	InterPrometheusTargetVersionRepo interface {
		List(tenantId string, targetId int64, page models.Page) ([]models.PrometheusTargetVersion, int64, error)
		Get(tenantId string, id int64) (models.PrometheusTargetVersion, error)
		Create(req *models.PrometheusTargetVersion) error
	}
)

func newPrometheusTargetVersionInterface(db *gorm.DB, g InterGormDBCli) InterPrometheusTargetVersionRepo {
	return &PrometheusTargetVersionRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

func (r PrometheusTargetVersionRepo) List(tenantId string, targetId int64, page models.Page) ([]models.PrometheusTargetVersion, int64, error) {
	var (
		data  []models.PrometheusTargetVersion
		db    = r.db.Model(&models.PrometheusTargetVersion{})
		count int64
	)

	db.Where("tenant_id = ? AND target_id = ?", tenantId, targetId)
	db.Count(&count)
	db.Order("version DESC")
	db.Limit(int(page.Size)).Offset(int((page.Index - 1) * page.Size))

	err := db.Find(&data).Error
	if err != nil {
		return nil, 0, err
	}
	return data, count, nil
}

func (r PrometheusTargetVersionRepo) Get(tenantId string, id int64) (models.PrometheusTargetVersion, error) {
	var data models.PrometheusTargetVersion
	err := r.db.Model(&models.PrometheusTargetVersion{}).
		Where("tenant_id = ? AND id = ?", tenantId, id).
		First(&data).Error
	return data, err
}

func (r PrometheusTargetVersionRepo) Create(req *models.PrometheusTargetVersion) error {
	var maxVersion int
	r.db.Model(&models.PrometheusTargetVersion{}).
		Where("tenant_id = ? AND target_id = ?", req.TenantId, req.TargetId).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion)
	req.Version = maxVersion + 1
	return r.g.Create(&models.PrometheusTargetVersion{}, req)
}

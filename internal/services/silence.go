package services

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"watchAlert/internal/ctx"
	models "watchAlert/internal/models"
	"watchAlert/internal/repo"
	"watchAlert/internal/types"
	"watchAlert/pkg/tools"
)

type alertSilenceService struct {
	alertEvent models.AlertCurEvent
	ctx        *ctx.Context
	writeMu    sync.Mutex
}

type InterSilenceService interface {
	Create(req interface{}) (interface{}, interface{})
	Update(req interface{}) (interface{}, interface{})
	Delete(req interface{}) (interface{}, interface{})
	List(req interface{}) (interface{}, interface{})
}

func newInterSilenceService(ctx *ctx.Context) InterSilenceService {
	return &alertSilenceService{
		ctx: ctx,
	}
}

func (ass *alertSilenceService) Create(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestSilenceCreate)
	updateAt := time.Now().Unix()
	silence := models.AlertSilences{
		TenantId:      r.TenantId,
		Name:          r.Name,
		ID:            "s-" + tools.RandId(),
		StartsAt:      r.StartsAt,
		EndsAt:        r.EndsAt,
		UpdateAt:      updateAt,
		UpdateBy:      r.UpdateBy,
		FaultCenterId: r.FaultCenterId,
		Labels:        r.Labels,
		Comment:       r.Comment,
		Status:        1,
	}

	if err := silence.Validate(updateAt); err != nil {
		return nil, err
	}
	silence.Status = silence.StatusAt(updateAt)

	err := ass.persist(silence.TenantId, silence.ID, false, func(store repo.InterSilenceRepo) error {
		return store.Create(silence)
	}, func() error { return ass.ctx.Redis.Silence().PushAlertMute(silence) })
	return nil, err
}

func (ass *alertSilenceService) Update(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestSilenceUpdate)
	silence := models.AlertSilences{
		TenantId:      r.TenantId,
		Name:          r.Name,
		ID:            r.ID,
		StartsAt:      r.StartsAt,
		EndsAt:        r.EndsAt,
		UpdateAt:      time.Now().Unix(),
		UpdateBy:      r.UpdateBy,
		FaultCenterId: r.FaultCenterId,
		Labels:        r.Labels,
		Comment:       r.Comment,
		Status:        1,
	}

	if err := silence.Validate(silence.UpdateAt); err != nil {
		return nil, err
	}
	silence.Status = silence.StatusAt(silence.UpdateAt)

	err := ass.persist(silence.TenantId, silence.ID, true, func(store repo.InterSilenceRepo) error {
		return store.Update(silence)
	}, func() error { return ass.ctx.Redis.Silence().PushAlertMute(silence) })
	return nil, err
}

func (ass *alertSilenceService) Delete(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestSilenceQuery)
	err := ass.persist(r.TenantId, r.ID, true, func(store repo.InterSilenceRepo) error {
		return store.Delete(r.TenantId, r.ID)
	}, func() error { return ass.ctx.Redis.Silence().RemoveAlertMute(r.TenantId, r.ID) })
	return nil, err
}

// Keep database changes uncommitted until the matching cache accepts them.
// Restore the old cache value on a failed transaction, including an ambiguous
// Redis response or a database commit failure after a successful cache write.
func (ass *alertSilenceService) persist(tenantID, id string, existing bool, mutate func(repo.InterSilenceRepo) error, publish func() error) error {
	ass.writeMu.Lock()
	defer ass.writeMu.Unlock()
	var previous *models.AlertSilences
	cacheAttempted := false
	err := ass.ctx.DB.Silence().WithTransaction(func(store repo.InterSilenceRepo) error {
		if existing {
			var err error
			previous, err = store.Get(tenantID, id)
			if err != nil {
				return err
			}
		}
		if err := mutate(store); err != nil {
			return err
		}
		cacheAttempted = true
		if err := publish(); err != nil {
			return fmt.Errorf("静默缓存同步失败，操作未完成: %w", err)
		}
		return nil
	})
	if err != nil && cacheAttempted {
		var restoreErr error
		if previous == nil {
			restoreErr = ass.ctx.Redis.Silence().RemoveAlertMute(tenantID, id)
		} else {
			restoreErr = ass.ctx.Redis.Silence().PushAlertMute(*previous)
		}
		if restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("恢复静默缓存失败，请重试原操作: %w", restoreErr))
		}
	}
	return err
}

func (ass *alertSilenceService) List(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestSilenceQuery)
	data, count, err := ass.ctx.DB.Silence().List(r.TenantId, r.FaultCenterId, r.Query, r.Status, r.Page)
	if err != nil {
		return nil, err
	}

	return types.ResponseSilenceList{
		List: data,
		Page: models.Page{
			Total: count,
			Index: r.Page.Index,
			Size:  r.Page.Size,
		},
	}, nil
}

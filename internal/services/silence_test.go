package services

import (
	"errors"
	"testing"
	"time"
	"watchAlert/internal/cache"
	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
	"watchAlert/internal/repo"
	"watchAlert/internal/types"
)

type silenceServiceRepo struct {
	repo.InterEntryRepo
	silence *silenceServiceStore
}

func (r *silenceServiceRepo) Silence() repo.InterSilenceRepo { return r.silence }

type silenceServiceStore struct {
	repo.InterSilenceRepo
	saved     models.AlertSilences
	err       error
	commitErr error
}

func (r *silenceServiceStore) Create(s models.AlertSilences) error { r.saved = s; return r.err }
func (r *silenceServiceStore) Update(s models.AlertSilences) error { r.saved = s; return r.err }
func (r *silenceServiceStore) Delete(string, string) error         { return r.err }

func (r *silenceServiceStore) Get(string, string) (*models.AlertSilences, error) {
	copy := r.saved
	return &copy, r.err
}

func (r *silenceServiceStore) WithTransaction(fn func(repo.InterSilenceRepo) error) error {
	previous := r.saved
	err := fn(r)
	if err == nil {
		err = r.commitErr
	}
	if err != nil {
		r.saved = previous
	}
	return err
}

func TestSilenceServiceCacheFailureRollsBackAndCommitFailureRestoresCache(t *testing.T) {
	now := time.Now().Unix()
	old := models.AlertSilences{TenantId: "t", ID: "s", Name: "old", StartsAt: now, EndsAt: now + 600,
		Labels: []models.SilenceLabel{{Key: "instance", Value: "core", Operator: "=="}}}
	for _, op := range []string{"create", "update", "delete"} {
		t.Run(op, func(t *testing.T) {
			store := &silenceServiceStore{saved: old}
			c := &silenceServiceCache{saved: &old, writeErr: errors.New("Redis unavailable")}
			service := newInterSilenceService(&ctx.Context{DB: &silenceServiceRepo{silence: store}, Redis: c})
			var err interface{}
			switch op {
			case "create":
				_, err = service.Create(&types.RequestSilenceCreate{TenantId: "t", Name: "new", StartsAt: now, EndsAt: now + 600, Labels: old.Labels})
			case "update":
				_, err = service.Update(&types.RequestSilenceUpdate{TenantId: "t", ID: "s", Name: "new", StartsAt: now, EndsAt: now + 600, Labels: old.Labels})
			case "delete":
				_, err = service.Delete(&types.RequestSilenceQuery{TenantId: "t", ID: "s"})
			}
			if err == nil || store.saved.Name != "old" || c.saved.Name != "old" || c.removed {
				t.Fatal("cache failure was accepted or changed the previous state")
			}
		})
	}
	store := &silenceServiceStore{saved: old, commitErr: errors.New("commit failed")}
	c := &silenceServiceCache{saved: &old}
	service := newInterSilenceService(&ctx.Context{DB: &silenceServiceRepo{silence: store}, Redis: c})
	_, err := service.Update(&types.RequestSilenceUpdate{TenantId: "t", ID: "s", Name: "new", StartsAt: now, EndsAt: now + 600, Labels: old.Labels})
	if err == nil || store.saved.Name != "old" || c.saved.Name != "old" {
		t.Fatal("commit failure did not restore the old cache value")
	}
}

type silenceServiceCache struct {
	cache.InterEntryCache
	saved    *models.AlertSilences
	removed  bool
	writeErr error
}

func (c *silenceServiceCache) Silence() cache.SilenceCacheInterface { return c }
func (c *silenceServiceCache) PushAlertMute(s models.AlertSilences) error {
	if c.writeErr != nil {
		return c.writeErr
	}
	c.saved = &s
	return nil
}
func (c *silenceServiceCache) RemoveAlertMute(string, string) error {
	if c.writeErr != nil {
		return c.writeErr
	}
	c.removed = true
	c.saved = nil
	return nil
}
func (c *silenceServiceCache) GetAlertMutes(string) ([]string, error) { return nil, nil }
func (c *silenceServiceCache) WithIdGetMuteFromCache(string, string) (*models.AlertSilences, error) {
	return c.saved, nil
}

func TestSilenceServiceScheduleAndFailedWrites(t *testing.T) {
	store := &silenceServiceStore{}
	c := &silenceServiceCache{}
	service := newInterSilenceService(&ctx.Context{DB: &silenceServiceRepo{silence: store}, Redis: c})
	now := time.Now().Unix()
	req := &types.RequestSilenceCreate{TenantId: "t-1", StartsAt: now + 600, EndsAt: now + 1200, Labels: []models.SilenceLabel{{Key: "instance", Operator: "==", Value: "core"}}}
	if _, err := service.Create(req); err != nil {
		t.Fatal(err)
	}
	if c.saved.Status != 0 || store.saved.Status != 0 {
		t.Fatal("scheduled silence created active")
	}
	update := &types.RequestSilenceUpdate{TenantId: "t-1", ID: c.saved.ID, StartsAt: now + 900, EndsAt: now + 1800, UpdateAt: now + 9999, Labels: req.Labels}
	if _, err := service.Update(update); err != nil {
		t.Fatal(err)
	}
	if c.saved.Status != 0 {
		t.Fatal("update trusted client timestamp")
	}
	store.err = errors.New("database unavailable")
	c.saved = nil
	if _, err := service.Create(req); err == nil || c.saved != nil {
		t.Fatal("failed create published cache")
	}
	if _, err := service.Update(update); err == nil || c.saved != nil {
		t.Fatal("failed update published cache")
	}
	if _, err := service.Delete(&types.RequestSilenceQuery{TenantId: "t-1", ID: "s-1"}); err == nil || c.removed {
		t.Fatal("failed delete removed cache")
	}
	req.Labels = nil
	store.err = nil
	if _, err := service.Create(req); err == nil {
		t.Fatal("empty conditions accepted")
	}
}

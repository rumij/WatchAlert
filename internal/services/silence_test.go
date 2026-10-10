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
	saved models.AlertSilences
	err   error
}

func (r *silenceServiceStore) Create(s models.AlertSilences) error { r.saved = s; return r.err }
func (r *silenceServiceStore) Update(s models.AlertSilences) error { r.saved = s; return r.err }
func (r *silenceServiceStore) Delete(string, string) error         { return r.err }

type silenceServiceCache struct {
	cache.InterEntryCache
	saved   *models.AlertSilences
	removed bool
}

func (c *silenceServiceCache) Silence() cache.SilenceCacheInterface   { return c }
func (c *silenceServiceCache) PushAlertMute(s models.AlertSilences)   { c.saved = &s }
func (c *silenceServiceCache) RemoveAlertMute(string, string)         { c.removed = true }
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

package repo

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
	"watchAlert/internal/models"
)

func TestRescheduleSilencePersistsZeroValuesAndScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err = db.AutoMigrate(&models.AlertSilences{}); err != nil {
		t.Fatal(err)
	}
	repo := newSilenceInterface(db, NewInterGormDBCli(db))
	rule := models.AlertSilences{TenantId: "t-1", ID: "s-1", Status: 1, StartsAt: 100, EndsAt: 200, Comment: "old", Labels: []models.SilenceLabel{{Key: "instance", Operator: "==", Value: "core"}}}
	if err = repo.Create(rule); err != nil {
		t.Fatal(err)
	}
	rule.Status = 0
	rule.StartsAt = 300
	rule.EndsAt = 400
	rule.Comment = ""
	rule.Labels[0].Value = "edge"
	if err = repo.Update(rule); err != nil {
		t.Fatal(err)
	}
	var saved models.AlertSilences
	if err = db.First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != 0 || saved.Comment != "" || saved.StartsAt != 300 || saved.Labels[0].Value != "edge" {
		t.Fatalf("reschedule not persisted: %+v", saved)
	}
	rule.TenantId = "t-2"
	if repo.Update(rule) == nil {
		t.Fatal("other tenant update accepted")
	}
}

func TestSilenceListDerivesStatusWithoutBackgroundWrites(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err = db.AutoMigrate(&models.AlertSilences{}); err != nil {
		t.Fatal(err)
	}
	repo := newSilenceInterface(db, NewInterGormDBCli(db))
	now := time.Now().Unix()
	for _, s := range []models.AlertSilences{
		{TenantId: "t", ID: "future", StartsAt: now + 600, EndsAt: now + 1200, Status: 1},
		{TenantId: "t", ID: "active", StartsAt: now - 600, EndsAt: now + 1200, Status: 0},
		{TenantId: "t", ID: "expired", StartsAt: now - 600, EndsAt: now - 10, Status: 1},
		{TenantId: "other", ID: "other", StartsAt: now - 600, EndsAt: now - 10, Status: 1},
	} {
		if err = repo.Create(s); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		status, id string
		state      int
	}{{"0", "future", 0}, {"1", "active", 1}, {"2", "expired", 2}} {
		rows, count, err := repo.List("t", "", "", tc.status, models.Page{Index: 1, Size: 10})
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 || len(rows) != 1 || rows[0].ID != tc.id || rows[0].Status != tc.state {
			t.Fatalf("status %s returned %+v (count=%d)", tc.status, rows, count)
		}
	}
}

func TestSilenceSearchByNameIsScopedAndOrdered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err = db.AutoMigrate(&models.AlertSilences{}); err != nil {
		t.Fatal(err)
	}
	repo := newSilenceInterface(db, NewInterGormDBCli(db))
	for _, rule := range []models.AlertSilences{
		{TenantId: "t", FaultCenterId: "fc", ID: "older", Name: "批量维护", UpdateAt: 10},
		{TenantId: "t", FaultCenterId: "fc", ID: "newer", Name: "批量维护", UpdateAt: 20},
		{TenantId: "other", FaultCenterId: "fc", ID: "other", Name: "批量维护", UpdateAt: 30},
		{TenantId: "t", FaultCenterId: "other", ID: "other-fc", Comment: "批量维护", UpdateAt: 40},
	} {
		if err = repo.Create(rule); err != nil {
			t.Fatal(err)
		}
	}
	rows, count, err := repo.List("t", "fc", "批量维护", "all", models.Page{Index: 1, Size: 1})
	if err != nil || count != 2 || len(rows) != 1 || rows[0].ID != "newer" {
		t.Fatalf("unexpected first page: %+v count=%d err=%v", rows, count, err)
	}
	rows, count, err = repo.List("t", "fc", "批量维护", "all", models.Page{Index: 2, Size: 1})
	if err != nil || count != 2 || len(rows) != 1 || rows[0].ID != "older" {
		t.Fatalf("unexpected second page: %+v count=%d err=%v", rows, count, err)
	}
}

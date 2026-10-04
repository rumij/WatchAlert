package zabbix

import (
	"context"
	"os"
	"testing"
	"time"
	appctx "watchAlert/internal/ctx"
	"watchAlert/internal/models"
	"watchAlert/internal/repo"
	"watchAlert/pkg/provider"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestLiveZabbixSync exercises the real API without writing to WatchAlert's
// database or Redis. It runs only when both test environment variables are set.
func TestLiveZabbixSync(t *testing.T) {
	token := os.Getenv("WATCHALERT_ZABBIX_TEST_TOKEN")
	endpoint := os.Getenv("WATCHALERT_ZABBIX_TEST_URL")
	if token == "" || endpoint == "" {
		t.Skip("set WATCHALERT_ZABBIX_TEST_TOKEN and WATCHALERT_ZABBIX_TEST_URL for the live test")
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	if err := db.Exec("CREATE TABLE alert_rules (tenant_id TEXT, datasource_type TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ZabbixSyncState{}, &models.AlertHisEvent{}); err != nil {
		t.Fatal(err)
	}

	datasource := models.AlertDataSource{
		TenantId: "live-test-tenant", ID: "live-test-datasource", Type: provider.ZabbixDsProviderName,
		HTTP: models.HTTP{URL: endpoint},
		ZabbixConfig: models.DsZabbixConfig{
			APIToken: token, FaultCenterId: "live-test-fault-center", InitialLookback: 3600,
		},
	}
	client, err := provider.NewZabbixClient(datasource)
	if err != nil {
		t.Fatal(err)
	}
	alerts := &zabbixTestAlertCache{events: make(map[string]*models.AlertCurEvent)}
	s := &Syncer{
		ctx: &appctx.Context{
			Ctx:   context.Background(),
			DB:    &liveZabbixTestRepo{zabbixTestRepo: zabbixTestRepo{db: db}},
			Redis: &zabbixTestEntryCache{alerts: alerts},
		},
		status: make(map[string]SyncStatus),
	}
	s.syncOnce(context.Background(), datasource, client)
	status := s.Status(datasource.ID)
	if status.LastError != "" || status.LastSuccessAt == 0 {
		t.Fatalf("live Zabbix sync failed: %s", status.LastError)
	}
	if status.ActiveProblems == 0 || status.ActiveProblems != len(alerts.events) {
		t.Fatalf("active problem count does not match the isolated cache: active=%d cached=%d", status.ActiveProblems, len(alerts.events))
	}
	for fingerprint, event := range alerts.events {
		if event.IsRecovered || event.Labels["source"] != "zabbix" || event.Labels["zabbix_eventid"] == "" {
			t.Fatalf("invalid synchronized event at %s", fingerprint)
		}
	}
	var state models.ZabbixSyncState
	if err := db.First(&state, "datasource_id = ?", datasource.ID).Error; err != nil || state.LastEventSyncAt == 0 {
		t.Fatalf("sync watermark was not saved: %v", err)
	}
	var historyCount int64
	if err := db.Model(&models.AlertHisEvent{}).Count(&historyCount).Error; err != nil {
		t.Fatal(err)
	}
	if historyCount != int64(status.HistoryBackfilled) {
		t.Fatalf("history count mismatch: saved=%d reported=%d", historyCount, status.HistoryBackfilled)
	}
	t.Logf("live sync: active=%d history_backfilled=%d", status.ActiveProblems, historyCount)
	s.syncOnce(context.Background(), datasource, client)
	secondStatus := s.Status(datasource.ID)
	if secondStatus.LastError != "" || secondStatus.LastSuccessAt == 0 {
		t.Fatalf("second live Zabbix sync failed: %s", secondStatus.LastError)
	}
	var secondHistoryCount, uniqueHistoryCount int64
	if err := db.Model(&models.AlertHisEvent{}).Count(&secondHistoryCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.AlertHisEvent{}).Distinct("event_id").Count(&uniqueHistoryCount).Error; err != nil {
		t.Fatal(err)
	}
	if secondHistoryCount != uniqueHistoryCount || secondHistoryCount-historyCount != int64(secondStatus.HistoryBackfilled) {
		t.Fatalf("history backfill duplicated records: total=%d unique=%d new=%d", secondHistoryCount, uniqueHistoryCount, secondStatus.HistoryBackfilled)
	}
	t.Logf("second sync: active=%d recovered=%d new_history=%d", secondStatus.ActiveProblems, secondStatus.RecoveredProblems, secondStatus.HistoryBackfilled)

	recent, err := client.GetProblemEventsSince(context.Background(), time.Now().Unix()-3600)
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range recent {
		if problem.RecoveryEventID == "" || problem.RecoveryEventID == "0" {
			continue
		}
		recoveryTimes, err := s.getRecoveryTimes(context.Background(), client, []string{problem.EventID})
		if err != nil || recoveryTimes[problem.EventID] <= 0 {
			t.Fatalf("live recovery lookup failed: %v", err)
		}
		fingerprint := routeFingerprint(datasource.ID, problem.EventID, nil)
		cached := &models.AlertCurEvent{
			TenantId: datasource.TenantId, EventId: "zbx-" + datasource.ID + "-" + problem.EventID,
			DatasourceType: provider.ZabbixDsProviderName, DatasourceId: datasource.ID,
			RuleGroupId: "zabbix", Fingerprint: fingerprint, Status: models.StateAlerting,
			Labels: map[string]interface{}{"zabbix_eventid": problem.EventID},
		}
		alerts.events[fingerprint] = cached
		s.syncOnce(context.Background(), datasource, client)
		thirdStatus := s.Status(datasource.ID)
		if thirdStatus.LastError != "" || thirdStatus.RecoveredProblems < 1 || !cached.IsRecovered || cached.RecoverTime != recoveryTimes[problem.EventID] {
			t.Fatalf("live recovery transition failed: recovered=%v time=%d expected=%d status=%+v", cached.IsRecovered, cached.RecoverTime, recoveryTimes[problem.EventID], thirdStatus)
		}
		t.Log("live recovery transition and timestamp verified")
		return
	}
	t.Log("no recovered event in the last hour; recovery transition is covered by the local lifecycle test")
}

type liveZabbixTestRepo struct {
	zabbixTestRepo
}

func (r *liveZabbixTestRepo) Event() repo.InterEventRepo {
	return &liveZabbixTestEventRepo{db: r.db}
}

type liveZabbixTestEventRepo struct {
	repo.InterEventRepo
	db *gorm.DB
}

func (r *liveZabbixTestEventRepo) CreateHistoryEvent(event models.AlertHisEvent) error {
	return r.db.Create(&event).Error
}

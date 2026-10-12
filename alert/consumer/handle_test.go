package consumer

import (
	"testing"
	"time"
	"watchAlert/internal/cache"
	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
)

func TestAlarmAggregationDoesNotMarkEventsAsSentBeforeRouting(t *testing.T) {
	events := []*models.AlertCurEvent{
		{Fingerprint: "first", LastSendTime: 0},
		{Fingerprint: "second", LastSendTime: 0},
	}
	groups := map[string][]*models.AlertCurEvent{"P2": events}

	result := alarmAggregation("alarm", models.FaultCenter{AggregationType: "Rule"}, groups)

	if len(result["P2"]) != 1 {
		t.Fatalf("expected one aggregated event, got %d", len(result["P2"]))
	}
	for _, event := range events {
		if event.LastSendTime != 0 {
			t.Fatalf("event %s was marked as sent before notification routing", event.Fingerprint)
		}
	}
}

func TestGetNoticeRoutesMatchesZabbixMappedSeverity(t *testing.T) {
	notice := models.AlertNotice{Routes: []models.Route{
		{NoticeType: "FeiShu", Severitys: []string{"P0", "P1", "P2"}},
	}}

	for _, severity := range []string{"P0", "P1", "P2"} {
		if routes := getNoticeRoutes(notice, severity); len(routes) != 1 {
			t.Fatalf("expected route for %s, got %d", severity, len(routes))
		}
	}
	if routes := getNoticeRoutes(notice, "P3"); len(routes) != 0 {
		t.Fatalf("did not expect route for P3, got %d", len(routes))
	}
}

type silenceAggregationCache struct {
	cache.InterEntryCache
	rule models.AlertSilences
}

func (c *silenceAggregationCache) Silence() cache.SilenceCacheInterface { return c }
func (c *silenceAggregationCache) GetAlertMutes(string) ([]string, error) {
	return []string{"s-1"}, nil
}
func (c *silenceAggregationCache) WithIdGetMuteFromCache(string, string) (*models.AlertSilences, error) {
	return &c.rule, nil
}
func (c *silenceAggregationCache) PushAlertMute(models.AlertSilences) error { return nil }
func (c *silenceAggregationCache) RemoveAlertMute(string, string) error     { return nil }

func TestSingleSilenceBeforeAggregationAndUpgrade(t *testing.T) {
	now := time.Now().Unix()
	c := &silenceAggregationCache{rule: models.AlertSilences{TenantId: "t", FaultCenterId: "fc", StartsAt: now - 60, EndsAt: now + 600, Labels: []models.SilenceLabel{{Key: "watchalert_fingerprint", Operator: "==", Value: "muted"}}}}
	old := ctx.Redis
	ctx.Redis = c
	t.Cleanup(func() { ctx.Redis = old })
	fc := models.FaultCenter{ID: "fc", TenantId: "t", AggregationType: "Rule", UpgradableSeverity: []string{"P1"}}
	muted := &models.AlertCurEvent{TenantId: "t", FaultCenterId: "fc", Fingerprint: "muted", Severity: "P1", Status: models.StateAlerting}
	other := &models.AlertCurEvent{TenantId: "t", FaultCenterId: "fc", Fingerprint: "other", Severity: "P1", Status: models.StateAlerting}
	for _, events := range [][]*models.AlertCurEvent{{muted, other}, {other, muted}} {
		groups := groupUnmutedAlerts(fc, events)
		result := alarmAggregation("alarm", fc, groups)
		if len(groups["P1"]) != 1 || len(result["P1"]) != 1 || result["P1"][0].Fingerprint != "other" {
			t.Fatal("single silence affected unrelated event or leaked into aggregation")
		}
		if muted.LastSendTime != 0 {
			t.Fatal("muted alert marked sent")
		}
	}
	if got := filterAlertEvents(fc, map[string]*models.AlertCurEvent{"muted": muted, "other": other}); len(got) != 1 || got[0].Fingerprint != "other" {
		t.Fatal("upgrade ignored silence")
	}
	c.rule.EndsAt = now
	if len(groupUnmutedAlerts(fc, []*models.AlertCurEvent{muted, other})["P1"]) != 2 {
		t.Fatal("events not eligible after expiry")
	}
}

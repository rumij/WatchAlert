package consumer

import (
	"testing"
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

package mute

import (
	"testing"
	"time"
	"watchAlert/internal/cache"
	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
)

type muteTestCache struct {
	cache.InterEntryCache
	rule models.AlertSilences
}

func (c *muteTestCache) Silence() cache.SilenceCacheInterface   { return c }
func (c *muteTestCache) GetAlertMutes(string) ([]string, error) { return []string{"s-1"}, nil }
func (c *muteTestCache) WithIdGetMuteFromCache(string, string) (*models.AlertSilences, error) {
	return &c.rule, nil
}
func (c *muteTestCache) PushAlertMute(models.AlertSilences) {}
func (c *muteTestCache) RemoveAlertMute(string, string)     {}

func TestEventSilenceScopeAndTime(t *testing.T) {
	now := time.Now().Unix()
	c := &muteTestCache{rule: models.AlertSilences{TenantId: "t-1", FaultCenterId: "fc-1", StartsAt: now - 10, EndsAt: now + 600, Status: 0, Labels: []models.SilenceLabel{{Key: "watchalert_fingerprint", Operator: "==", Value: "one"}}}}
	old := ctx.Redis
	ctx.Redis = c
	t.Cleanup(func() { ctx.Redis = old })
	event := models.AlertCurEvent{TenantId: "t-1", FaultCenterId: "fc-1", Fingerprint: "one", Labels: map[string]interface{}{"watchalert_fingerprint": "spoofed"}}
	if !IsSilence(ParamsForEvent(&event, nil)) {
		t.Fatal("active single event must be muted despite stale status")
	}
	if event.Labels["watchalert_fingerprint"] != "spoofed" {
		t.Fatal("event labels mutated")
	}
	event.Fingerprint = "two"
	if IsSilence(ParamsForEvent(&event, nil)) {
		t.Fatal("unrelated event muted")
	}
	event.Fingerprint = "one"
	event.FaultCenterId = "fc-2"
	if IsSilence(ParamsForEvent(&event, nil)) {
		t.Fatal("other fault center muted")
	}
	event.FaultCenterId = "fc-1"
	event.TenantId = "t-2"
	if IsSilence(ParamsForEvent(&event, nil)) {
		t.Fatal("other tenant muted")
	}
	event.TenantId = "t-1"
	c.rule.Status = 1
	c.rule.StartsAt = now + 300
	if IsSilence(ParamsForEvent(&event, nil)) {
		t.Fatal("scheduled silence began early")
	}
	c.rule.StartsAt = now - 300
	c.rule.EndsAt = now
	if IsSilence(ParamsForEvent(&event, nil)) {
		t.Fatal("expired silence remained active")
	}
}

func TestClassSelectorsAndScalarValues(t *testing.T) {
	labels := map[string]interface{}{"name": "Interface Gi1: Link down", "suppressed": false, "count": float64(2)}
	for _, tc := range []struct {
		name, key, op, value string
		want                 bool
	}{
		{"keyword", "name", "contains", "link down", true},
		{"literal keyword", "name", "contains", "[", false},
		{"exclude keyword", "name", "!contains", "CPU", true},
		{"class", "name", "=~", "Link down", true},
		{"other class", "name", "=~", "CPU", false},
		{"boolean mismatch", "suppressed", "==", "true", false},
		{"boolean match", "suppressed", "==", "false", true},
		{"numeric mismatch", "count", "==", "3", false},
		{"numeric match", "count", "==", "2", true},
		{"invalid regexp", "name", "=~", "[", false},
		{"invalid negative regexp", "name", "!~", "[", false},
		{"missing", "missing", "!=", "anything", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := evalCondition(labels, []models.SilenceLabel{{Key: tc.key, Operator: tc.op, Value: tc.value}})
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	if evalCondition(labels, nil) {
		t.Fatal("empty selector silenced everything")
	}
	if evalCondition(labels, []models.SilenceLabel{{Key: "name", Operator: "=~", Value: "Link down"}, {Key: "count", Operator: "==", Value: "3"}}) {
		t.Fatal("conditions must be ANDed")
	}
}

package models

import (
	"encoding/json"
	"testing"
)

func TestSilenceLabelsAcceptLegacyZabbixScalars(t *testing.T) {
	var labels []SilenceLabel
	input := `[{"key":"zabbix_item_count","operator":"=","value":3},{"key":"zabbix_suppressed","operator":"=","value":true},{"key":"zabbix_triggerid","operator":"=","value":"82401"}]`
	if err := json.Unmarshal([]byte(input), &labels); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"3", "true", "82401"} {
		if labels[i].Value != want {
			t.Fatalf("label %d value = %q, want %q", i, labels[i].Value, want)
		}
	}
	for _, invalid := range []string{`{"key":"x","operator":"=","value":null}`, `{"key":"x","operator":"=","value":[]}`} {
		if err := json.Unmarshal([]byte(invalid), new(SilenceLabel)); err == nil {
			t.Fatalf("accepted invalid condition value: %s", invalid)
		}
	}
}

func TestSilenceWindowAndValidation(t *testing.T) {
	s := AlertSilences{StartsAt: 100, EndsAt: 200, Labels: []SilenceLabel{{Key: "instance", Operator: "=", Value: "core"}}}
	for _, tc := range []struct {
		now  int64
		want int
	}{{99, 0}, {100, 1}, {199, 1}, {200, 2}, {201, 2}} {
		if got := s.StatusAt(tc.now); got != tc.want {
			t.Errorf("now=%d got=%d want=%d", tc.now, got, tc.want)
		}
	}
	if err := s.Validate(90); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*AlertSilences)
	}{
		{"reversed", func(s *AlertSilences) { s.EndsAt = 99 }},
		{"expired", func(s *AlertSilences) { s.EndsAt = 150 }},
		{"empty conditions", func(s *AlertSilences) { s.Labels = nil }},
		{"empty value", func(s *AlertSilences) { s.Labels = []SilenceLabel{{Key: "instance", Operator: "==", Value: ""}} }},
		{"blank regex", func(s *AlertSilences) { s.Labels = []SilenceLabel{{Key: "instance", Operator: "=~", Value: "  "}} }},
		{"invalid regex", func(s *AlertSilences) { s.Labels = []SilenceLabel{{Key: "instance", Operator: "=~", Value: "["}} }},
		{"unknown operator", func(s *AlertSilences) {
			s.Labels = []SilenceLabel{{Key: "instance", Operator: "unsupported", Value: "core"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := s
			tc.edit(&bad)
			if bad.Validate(160) == nil {
				t.Fatal("invalid silence accepted")
			}
		})
	}
}

package models

import "testing"

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

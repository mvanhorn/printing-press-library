// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package risk

import "testing"

func TestLookupMetric(t *testing.T) {
	cases := []struct {
		in      string
		wantKey string
		wantOK  bool
	}{
		{"heat", "heat", true},
		{"Heat", "heat", true},
		{"risk-per-trade", "heat", true},
		{"portfolio_heat", "heat", true},
		{"kelly", "kelly", true},
		{"optimal-f", "kelly", true},
		{"K", "kelly", true},
		{"uncle-point", "uncle-point", true},
		{"unclepoint", "uncle-point", true},
		{"lake-ratio", "lake-ratio", true},
		{"coin-toss", "coin-toss", true},
		{"fixed-fraction", "coin-toss", true},
		{"timid-bold", "timid-bold", true},
		{"bold", "timid-bold", true},
		{"nonsense", "", false},
	}
	for _, c := range cases {
		m, ok := LookupMetric(c.in)
		if ok != c.wantOK {
			t.Errorf("LookupMetric(%q): ok = %v; want %v", c.in, ok, c.wantOK)
			continue
		}
		if ok && m.Key != c.wantKey {
			t.Errorf("LookupMetric(%q) = %q; want %q", c.in, m.Key, c.wantKey)
		}
		if ok {
			if m.Name == "" || m.EssaySection == "" || m.Blurb == "" {
				t.Errorf("LookupMetric(%q) returned an under-populated metric: %+v", c.in, m)
			}
		}
	}
}

func TestMetricKeysAndAll(t *testing.T) {
	keys := MetricKeys()
	if len(keys) < 5 {
		t.Errorf("MetricKeys returned %d; want >= 5", len(keys))
	}
	all := AllMetrics()
	if len(all) != len(keys) {
		t.Errorf("AllMetrics (%d) and MetricKeys (%d) disagree", len(all), len(keys))
	}
	// every key resolves back to a metric
	for _, k := range keys {
		if _, ok := LookupMetric(k); !ok {
			t.Errorf("MetricKeys returned %q but LookupMetric can't find it", k)
		}
	}
}

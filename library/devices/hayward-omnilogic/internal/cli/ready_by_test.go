package cli

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/omnilogic"
	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/store"
)

func ptrInt(i int) *int { return &i }

// TestPickWaterTempForReadyBy locks the sentinel-rejection behavior that
// keeps `ready-by` from treating Hayward's -1 "sensor not reading"
// marker as a real temperature. Regression for Greptile #3216365845.
func TestPickWaterTempForReadyBy(t *testing.T) {
	cases := []struct {
		name      string
		tele      *omnilogic.Telemetry
		caps      store.SiteCapabilities
		wantErr   bool
		wantTemp  int
		errIncl   []string
	}{
		{
			name: "valid reading passes through",
			tele: &omnilogic.Telemetry{BodiesOfWater: []omnilogic.TelemetryBOW{
				{WaterTemp: ptrInt(78)},
			}},
			caps:     store.AssumeAllEquipped(1),
			wantTemp: 78,
		},
		{
			name: "Hayward -1 sentinel rejected with general hint",
			tele: &omnilogic.Telemetry{BodiesOfWater: []omnilogic.TelemetryBOW{
				{WaterTemp: ptrInt(-1)},
			}},
			caps:    store.AssumeAllEquipped(1),
			wantErr: true,
			errIncl: []string{"-1°F", "sentinel", "sync"},
		},
		{
			name: "Hayward -1 sentinel rejected with flow-needs hint when capabilities say so",
			tele: &omnilogic.Telemetry{BodiesOfWater: []omnilogic.TelemetryBOW{
				{WaterTemp: ptrInt(-1)},
			}},
			caps: store.SiteCapabilities{
				SiteMspSystemID: 1,
				TempNeedsFlow:   true,
			},
			wantErr: true,
			errIncl: []string{"temp_needs_flow", "Filter Pump", "30s"},
		},
		{
			name: "zero rejected (defensive — old guard caught only 0 anyway)",
			tele: &omnilogic.Telemetry{BodiesOfWater: []omnilogic.TelemetryBOW{
				{WaterTemp: ptrInt(0)},
			}},
			caps:    store.AssumeAllEquipped(1),
			wantErr: true,
			errIncl: []string{"0°F", "sentinel"},
		},
		{
			name:    "no reading at all yields 'no water temperature reading available'",
			tele:    &omnilogic.Telemetry{BodiesOfWater: []omnilogic.TelemetryBOW{{}}},
			caps:    store.AssumeAllEquipped(1),
			wantErr: true,
			errIncl: []string{"no water temperature reading available"},
		},
		{
			name:    "nil telemetry rejected with the same shape",
			tele:    nil,
			caps:    store.AssumeAllEquipped(1),
			wantErr: true,
			errIncl: []string{"no water temperature reading available"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := pickWaterTempForReadyBy(tc.tele, tc.caps)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got temp=%d", got)
				}
				for _, frag := range tc.errIncl {
					if !strings.Contains(err.Error(), frag) {
						t.Errorf("error %q missing fragment %q", err.Error(), frag)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantTemp {
				t.Errorf("temp: want %d, got %d", tc.wantTemp, got)
			}
		})
	}
}

// TestCommandLogNotAnnotatedReadOnly locks the safety contract on the
// command-log command: it must NOT carry the mcp:read-only annotation
// because --replay <id> dispatches live writes that physically control
// pool equipment. Regression for Greptile #3216424950 — MCP hosts use
// this annotation to decide when to prompt for permission; a false
// "read-only" claim would let an agent re-fire heater/pump commands
// without confirmation.
func TestCommandLogNotAnnotatedReadOnly(t *testing.T) {
	flags := &rootFlags{}
	cmd := newCommandLogCmd(flags)
	if v, ok := cmd.Annotations["mcp:read-only"]; ok {
		t.Errorf("command-log must NOT be annotated mcp:read-only (got %q) — --replay <id> issues live writes", v)
	}
}

// TestBuildDriftReport_TimestampAlignment locks the Greptile P1 #3216533851
// finding: `buildDriftReport` filters Hayward's -1 sentinels out of `values`
// but previously read forecast timestamps from the *unfiltered* `samples`
// slice. When the chronologically-earliest raw sample was a sentinel, the
// projected exit forecast divided a real reading delta by a span that
// stretched back into the sensor-offline period, producing a falsely *slow*
// rate of change.
//
// This test seeds the store with one legacy -1 sentinel at t-7d (the kind of
// row that exists in stores written before the AppendTelemetry filter
// landed), then three real pH readings between t-3d and now drifting toward
// the safe-high. With aligned timestamps, the forecast should describe an
// exit within roughly the 3-day real-data span; with the bug, the implied
// span would be ~7 days and the projected exit window would be off by ~2-3x.
func TestBuildDriftReport_TimestampAlignment(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "store.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	siteID := 1
	now := time.Now().UTC().Truncate(time.Second)
	insert := func(metric string, value float64, ts time.Time) {
		t.Helper()
		_, err := s.DB.Exec(
			`INSERT INTO telemetry_samples (site_msp_system_id, bow_system_id, metric, value_real, sampled_at)
			 VALUES (?, ?, ?, ?, ?)`,
			siteID, "10", metric, value, ts.Format(time.RFC3339),
		)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	// Legacy -1 sentinel at t-7d (mimics a pre-fix store row).
	insert("ph", -1, now.Add(-7*24*time.Hour))
	// Real readings drifting upward from 7.0 to 7.75 over the last 3 days.
	insert("ph", 7.00, now.Add(-3*24*time.Hour))
	insert("ph", 7.40, now.Add(-1*24*time.Hour))
	insert("ph", 7.75, now)

	report := buildDriftReport(s, siteID, true)
	if len(report.Metrics) == 0 {
		t.Fatalf("expected at least one metric in drift report")
	}
	var ph *driftMetricReport
	for i := range report.Metrics {
		if report.Metrics[i].Metric == "ph" {
			ph = &report.Metrics[i]
			break
		}
	}
	if ph == nil {
		t.Fatalf("ph metric missing from drift report: %+v", report.Metrics)
	}
	// Samples count must reflect the *filtered* slice (3 real readings),
	// not the raw 4 with the sentinel.
	if ph.Samples != 3 {
		t.Errorf("expected 3 samples after sentinel filter, got %d", ph.Samples)
	}
	if !ph.Drifting {
		t.Fatalf("expected ph to be drifting (current=7.75 vs baseline ~7.0)")
	}
	// With aligned timestamps, projectExit computes rate = (7.75-7.0)/72h
	// ≈ 0.0104/h, and current=7.75 vs safe-high=7.8 implies exit in ~4.8h.
	// With the bug, projectExit would use t-7d as the firstTs, computing
	// rate = 0.75/168h ≈ 0.00446/h and exit in ~11h.
	// Pin both anchors loosely so the test isn't brittle to minor tweaks
	// in the forecast string format.
	if ph.ForecastNote == "" {
		t.Fatalf("expected a forecast note when drifting+forecast=true")
	}
	if !strings.Contains(ph.ForecastNote, "exits high") {
		t.Errorf("forecast note should describe a high-exit; got %q", ph.ForecastNote)
	}
	// Extract the hours value and assert it's in the fixed-timing band
	// (≈ 4-6h), not the bug-timing band (≈ 10-12h).
	hours, err := parseForecastHours(ph.ForecastNote)
	if err != nil {
		t.Fatalf("could not parse hours from %q: %v", ph.ForecastNote, err)
	}
	if hours > 8 {
		t.Errorf("forecast hours %.2f suggests projectExit saw the legacy -1 timestamp; want <= 8h", hours)
	}
}

// TestBuildDriftReport_MonotonicityCheck locks the Greptile P1 #3228229129
// finding: the chemistry-drift Long description promises "AND the trend
// is monotonic over the last 5 samples" before flagging drift, but the
// original buildDriftReport only checked delta>threshold. A single tail
// outlier should NOT fire drifting=true if the prior samples don't show
// a sustained directional move — pool-service operators routing dispatch
// calls on these alerts can't afford false positives from transient spikes.
func TestBuildDriftReport_MonotonicityCheck(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "store.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	siteID := 1
	now := time.Now().UTC().Truncate(time.Second)
	insert := func(metric string, value float64, ts time.Time) {
		t.Helper()
		_, err := s.DB.Exec(
			`INSERT INTO telemetry_samples (site_msp_system_id, bow_system_id, metric, value_real, sampled_at)
			 VALUES (?, ?, ?, ?, ?)`,
			siteID, "10", metric, value, ts.Format(time.RFC3339),
		)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	// Six prior ORP readings averaging ~700 mV (cluster around baseline),
	// then a single tail-outlier of 760 mV. baseline = mean of first half =
	// ~700, delta = 760-700 = 60 > 50 threshold, so trend="up" — but the
	// SECOND-to-last sample (700) is BELOW the last (760), and the THIRD
	// (700) is equal to the second, breaking strict monotonicity.
	insert("orp", 700, now.Add(-7*24*time.Hour))
	insert("orp", 705, now.Add(-6*24*time.Hour))
	insert("orp", 695, now.Add(-5*24*time.Hour))
	insert("orp", 702, now.Add(-4*24*time.Hour))
	insert("orp", 698, now.Add(-3*24*time.Hour))
	insert("orp", 700, now.Add(-2*24*time.Hour))
	insert("orp", 760, now) // single tail outlier — should NOT trip drifting

	report := buildDriftReport(s, siteID, false)
	var orp *driftMetricReport
	for i := range report.Metrics {
		if report.Metrics[i].Metric == "orp" {
			orp = &report.Metrics[i]
			break
		}
	}
	if orp == nil {
		t.Fatalf("orp metric missing from drift report")
	}
	// Delta crosses threshold so trend reads "up", but a single tail outlier
	// against a flat-then-spike pattern is NOT monotonic over the last 5
	// samples (a single sample IS trivially monotonic; the prior cluster
	// going down/flat/up breaks it). Verify drifting stays false.
	if orp.Trend != "up" {
		t.Errorf("expected trend=up given delta of 60 > 50; got %q", orp.Trend)
	}
	if orp.Drifting {
		t.Errorf("expected drifting=false for non-monotonic tail outlier (samples=%+v); got drifting=true", orp.Samples)
	}

	// Sanity check: a real sustained drift (5 monotonically-rising samples)
	// should still trip drifting=true. Wipe and rebuild.
	if _, err := s.DB.Exec(`DELETE FROM telemetry_samples WHERE metric = 'orp'`); err != nil {
		t.Fatalf("delete: %v", err)
	}
	insert("orp", 700, now.Add(-7*24*time.Hour))
	insert("orp", 705, now.Add(-6*24*time.Hour))
	insert("orp", 720, now.Add(-5*24*time.Hour))
	insert("orp", 735, now.Add(-4*24*time.Hour))
	insert("orp", 745, now.Add(-3*24*time.Hour))
	insert("orp", 755, now.Add(-2*24*time.Hour))
	insert("orp", 770, now)

	report = buildDriftReport(s, siteID, false)
	for i := range report.Metrics {
		if report.Metrics[i].Metric == "orp" {
			orp = &report.Metrics[i]
			break
		}
	}
	if !orp.Drifting {
		t.Errorf("expected drifting=true for a sustained monotonic rise (last 5: 735,745,755,770); got false")
	}
}

// parseForecastHours pulls the float from a string like
// "at current rate, exits high (7.80) in 4.8 hours".
func parseForecastHours(note string) (float64, error) {
	const marker = ") in "
	i := strings.Index(note, marker)
	if i < 0 {
		return 0, strconv.ErrSyntax
	}
	rest := note[i+len(marker):]
	end := strings.Index(rest, " hours")
	if end < 0 {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseFloat(rest[:end], 64)
}

// TestMustBeReadOnlySQL_WordBoundary covers the Greptile #3216464122 finding
// that the original `strings.Contains(lower, banned+" ")` guard was bypassed
// by newline-separated keywords. Every banned op must be caught regardless
// of the trailing whitespace character (space / tab / newline / EOF).
func TestMustBeReadOnlySQL_WordBoundary(t *testing.T) {
	cases := []struct {
		query string
		want  string // empty = clean
		label string
	}{
		{"select * from sites", "", "plain SELECT clean"},
		{"with cte as (select 1) select * from cte", "", "CTE clean"},
		{"delete from sites", "delete", "delete followed by space"},
		{"delete\nfrom sites", "delete", "delete followed by NEWLINE (the bug)"},
		{"delete\tfrom sites", "delete", "delete followed by TAB"},
		{"DELETE FROM sites", "delete", "uppercase still caught after lowercasing"},
		{"drop table x", "drop", "drop"},
		{"insert into x values (1)", "insert", "insert"},
		{"update x set a=1", "update", "update"},
		{"alter table x add column y int", "alter", "alter"},
		{"create table z (a int)", "create", "create"},
		{"attach database '/x.db' as foo", "attach", "attach"},
		// Word-boundary correctness: these should NOT trigger.
		{"select created_at from sites", "", "create as substring of 'created_at' must not trigger"},
		{"select * from updates", "", "update as substring of 'updates' must not trigger"},
		{"select inserted_id from log", "", "insert as substring of 'inserted_id' must not trigger"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := mustBeReadOnlySQL(strings.ToLower(tc.query))
			if got != tc.want {
				t.Errorf("query %q: want %q, got %q", tc.query, tc.want, got)
			}
		})
	}
}

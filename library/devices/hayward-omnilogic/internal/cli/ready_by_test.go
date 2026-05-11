package cli

import (
	"strings"
	"testing"

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

// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored construction tests for the SRAM AXS novel commands.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Each novel command must build, expose the expected Use name, and exit 0
// under --dry-run without touching the network or the local store.
func TestNovelCommandsDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	checks := []struct {
		name string
		run  func() error
	}{
		{"firmware-check", func() error { c := newNovelFirmwareCheckCmd(flags); return c.RunE(c, nil) }},
		{"wear", func() error { c := newNovelWearCmd(flags); return c.RunE(c, nil) }},
		{"shifts", func() error { c := newNovelShiftsCmd(flags); return c.RunE(c, nil) }},
		{"battery", func() error { c := newNovelBatteryCmd(flags); return c.RunE(c, nil) }},
		{"garage", func() error { c := newNovelGarageCmd(flags); return c.RunE(c, nil) }},
		{"since", func() error { c := newNovelSinceCmd(flags); return c.RunE(c, []string{"7d"}) }},
		{"auth-login", func() error { c := newAuthLoginCmd(flags); return c.RunE(c, nil) }},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err != nil {
				t.Fatalf("%s dry-run RunE returned error: %v", tc.name, err)
			}
		})
	}
}

func TestNovelCommandUseStrings(t *testing.T) {
	flags := &rootFlags{}
	want := map[string]string{
		"firmware-check": newNovelFirmwareCheckCmd(flags).Use,
		"wear":           newNovelWearCmd(flags).Use,
		"shifts":         newNovelShiftsCmd(flags).Use,
		"battery":        newNovelBatteryCmd(flags).Use,
		"garage":         newNovelGarageCmd(flags).Use,
		"since [window]": newNovelSinceCmd(flags).Use,
		"login":          newAuthLoginCmd(flags).Use,
	}
	for expected, got := range want {
		if got != expected {
			t.Fatalf("Use = %q, want %q", got, expected)
		}
	}
}

func TestGarageRejectsLocalDataSource(t *testing.T) {
	flags := &rootFlags{dataSource: "local"}
	cmd := newNovelGarageCmd(flags)
	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("garage --data-source local returned nil error")
	}
	if !strings.Contains(err.Error(), "no local data source") {
		t.Fatalf("garage --data-source local error = %q, want no local data source", err.Error())
	}
}

func TestSinceRejectsLiveDataSource(t *testing.T) {
	flags := &rootFlags{dataSource: "live"}
	cmd := newNovelSinceCmd(flags)
	err := cmd.RunE(cmd, []string{"7d"})
	if err == nil {
		t.Fatal("since --data-source live returned nil error")
	}
	if !strings.Contains(err.Error(), "no live equivalent") {
		t.Fatalf("since --data-source live error = %q, want no live equivalent", err.Error())
	}
}

func TestParseAXSTimeEpochStrings(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want time.Time
	}{
		{"seconds", "1718000000", time.Unix(1718000000, 0).UTC()},
		{"milliseconds", "1718000000123", time.UnixMilli(1718000000123).UTC()},
		{"rfc3339", "2026-06-15T06:00:00Z", time.Date(2026, 6, 15, 6, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseAXSTime(tt.in)
			if !ok {
				t.Fatalf("parseAXSTime(%q) returned !ok", tt.in)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("parseAXSTime(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestSinceItemsSortByParsedTimestamp(t *testing.T) {
	items := []sinceItem{
		{Timestamp: "2024-06-01T00:00:00Z"},
		{Timestamp: "1718000000"},
		{Timestamp: "2024-06-09T00:00:00Z"},
	}
	sortSinceItems(items)
	if items[0].Timestamp != "1718000000" {
		t.Fatalf("first timestamp = %q, want epoch timestamp sorted by parsed time", items[0].Timestamp)
	}
}

func TestApplyWearShiftCountsDoesNotDoubleCountTotal(t *testing.T) {
	row := &wearRow{}
	applyWearShiftCounts(row, map[string]any{
		"shift_count":    float64(20),
		"fd_shift_count": float64(3),
	}, nil)
	if row.ShiftCount != 20 {
		t.Fatalf("ShiftCount = %v, want total shift_count without adding fd again", row.ShiftCount)
	}
	if row.FDShiftCount != 3 {
		t.Fatalf("FDShiftCount = %v, want 3", row.FDShiftCount)
	}

	row = &wearRow{}
	applyWearShiftCounts(row, map[string]any{
		"rd_shift_count": float64(17),
		"fd_shift_count": float64(3),
	}, nil)
	if row.ShiftCount != 20 || row.RDShiftCount != 17 || row.FDShiftCount != 3 {
		t.Fatalf("counts = total:%v rd:%v fd:%v, want 20/17/3", row.ShiftCount, row.RDShiftCount, row.FDShiftCount)
	}
}

func TestApplyWearShiftCountsFallsBackToItem(t *testing.T) {
	row := &wearRow{}
	applyWearShiftCounts(row, nil, map[string]any{
		"rd_shift_count": float64(17),
		"fd_shift_count": float64(3),
	})
	if row.ShiftCount != 20 || row.RDShiftCount != 17 || row.FDShiftCount != 3 {
		t.Fatalf("counts = total:%v rd:%v fd:%v, want item fallback 20/17/3", row.ShiftCount, row.RDShiftCount, row.FDShiftCount)
	}
}

func TestSummaryFirmwareVersionPreservesStringVersion(t *testing.T) {
	got, ok := summaryFirmwareVersion(summaryResource{
		Data: map[string]any{"fw_version": "2.47.0"},
	})
	if !ok || got != "2.47.0" {
		t.Fatalf("summaryFirmwareVersion() = %q, %v; want 2.47.0, true", got, ok)
	}
}

func TestSummaryFirmwareVersionAcceptsNumericFallback(t *testing.T) {
	got, ok := summaryFirmwareVersion(summaryResource{
		Item: map[string]any{"firmware_version": float64(12)},
	})
	if !ok || got != "12" {
		t.Fatalf("summaryFirmwareVersion() = %q, %v; want 12, true", got, ok)
	}
}

func TestWearBatteryStatusPreservesZero(t *testing.T) {
	row := &wearRow{}
	applyWearBatteryStatus(row, map[string]any{"battery_status": float64(0)}, nil)
	if row.BatteryStatus == nil || *row.BatteryStatus != 0 {
		t.Fatalf("BatteryStatus = %v, want pointer to zero", row.BatteryStatus)
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), `"battery_status":0`) {
		t.Fatalf("wear JSON = %s, want explicit zero battery_status", data)
	}
}

func TestLatestFirmwareRowsIgnoresNewerBatteryOnlySummary(t *testing.T) {
	rows := latestFirmwareRows([]summaryResource{
		{Data: map[string]any{"device_type": "1", "fw_version": "2.47.0", "end_ts": float64(10)}},
		{Data: map[string]any{"device_type": "1", "battery_status": float64(80), "end_ts": float64(20)}},
	})
	if len(rows) != 1 || rows[0].FirmwareVersion == nil || *rows[0].FirmwareVersion != "2.47.0" {
		t.Fatalf("latestFirmwareRows() = %#v, want retained firmware 2.47.0", rows)
	}
}

func TestLatestBatteryRowsRetainsIndependentLatestFields(t *testing.T) {
	rows := latestBatteryRows([]summaryResource{
		{Data: map[string]any{"device_type": "1", "voltage": float64(3.1), "end_ts": float64(10)}},
		{Data: map[string]any{"device_type": "1", "battery_status": float64(75), "end_ts": float64(20)}},
	})
	if len(rows) != 1 || rows[0].BatteryStatus == nil || *rows[0].BatteryStatus != 75 {
		t.Fatalf("latestBatteryRows() status = %#v, want 75", rows)
	}
	if rows[0].Voltage == nil || *rows[0].Voltage != 3.1 {
		t.Fatalf("latestBatteryRows() voltage = %#v, want retained 3.1", rows)
	}
}

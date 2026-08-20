// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestToFloatRC(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want float64
	}{
		{"float", float64(9.99), 9.99},
		{"int", 5, 5},
		{"numeric string", "12.5", 12.5},
		{"json.Number", json.Number("3.14"), 3.14},
		{"nil", nil, 0},
		{"garbage string", "abc", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := toFloatRC(tc.in); got != tc.want {
				t.Fatalf("toFloatRC(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMonetaryGrossUSD(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want float64
	}{
		{"monetary object", map[string]any{"currency": "USD", "gross": 19.99, "proceeds": 14.0}, 19.99},
		{"missing gross", map[string]any{"currency": "USD"}, 0},
		{"bare number fallback", float64(7.5), 7.5},
		{"nil", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := monetaryGrossUSD(tc.in); got != tc.want {
				t.Fatalf("monetaryGrossUSD(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestRCEpochMSToTime(t *testing.T) {
	// 1658399423658 ms => 2022-07-21T08:30:23.658Z
	got := rcEpochMSToTime(float64(1658399423658))
	if got.IsZero() {
		t.Fatal("expected non-zero time for valid epoch ms")
	}
	if got.Year() != 2022 {
		t.Fatalf("expected year 2022, got %d", got.Year())
	}
	if !rcEpochMSToTime(0).IsZero() {
		t.Fatal("expected zero time for 0 input")
	}
	if !rcEpochMSToTime(nil).IsZero() {
		t.Fatal("expected zero time for nil input")
	}
}

func TestChartDataPoints(t *testing.T) {
	cases := []struct {
		name        string
		values      []string // each a JSON row
		wantPoints  int
		wantFirst   float64
		wantHasTime bool
	}{
		{
			name: "single-measure cohort rows",
			values: []string{
				`{"cohort":1746057600,"incomplete":false,"measure":0,"value":100.5}`,
				`{"cohort":1748736000,"incomplete":false,"measure":0,"value":110.0}`,
			},
			wantPoints:  2,
			wantFirst:   100.5,
			wantHasTime: true,
		},
		{
			name: "multi-measure per cohort collapses to one point, measure 0 first",
			values: []string{
				`{"cohort":1746057600,"measure":0,"value":42}`,
				`{"cohort":1746057600,"measure":1,"value":7}`,
			},
			wantPoints:  1,
			wantFirst:   42,
			wantHasTime: true,
		},
		{
			name:       "empty",
			values:     nil,
			wantPoints: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cd := chartData{Resolution: "day"}
			for _, v := range tc.values {
				cd.Values = append(cd.Values, json.RawMessage(v))
			}
			pts := cd.points()
			if len(pts) != tc.wantPoints {
				t.Fatalf("points len = %d, want %d", len(pts), tc.wantPoints)
			}
			if tc.wantPoints == 0 {
				return
			}
			if tc.wantHasTime && pts[0].When.IsZero() {
				t.Fatalf("expected non-zero When on first point")
			}
			if tc.wantFirst != 0 && pts[0].firstSeriesValue() != tc.wantFirst {
				t.Fatalf("firstSeriesValue = %v, want %v", pts[0].firstSeriesValue(), tc.wantFirst)
			}
		})
	}
}

func TestSumFirstSeries(t *testing.T) {
	cd := chartData{}
	cd.Values = []json.RawMessage{
		json.RawMessage(`{"cohort":1746057600,"measure":0,"value":10}`),
		json.RawMessage(`{"cohort":1748736000,"measure":0,"value":20}`),
		json.RawMessage(`{"cohort":1751328000,"measure":0,"value":5}`),
	}
	if got := sumFirstSeries(cd); got != 35 {
		t.Fatalf("sumFirstSeries = %v, want 35", got)
	}
}

func TestPeriodKey(t *testing.T) {
	at := time.Date(2022, 7, 21, 8, 30, 0, 0, time.UTC)
	if got := periodKey(at); got != "2022-07-21" {
		t.Fatalf("periodKey = %q, want 2022-07-21", got)
	}
	if got := periodKey(time.Time{}); got != "(unknown)" {
		t.Fatalf("periodKey(zero) = %q, want (unknown)", got)
	}
}

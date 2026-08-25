// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral tests for the shared cross-entity novel-command helpers.

package cli

import (
	"testing"
	"time"
)

func TestNormTicker(t *testing.T) {
	cases := map[string]string{
		"$NVDA": "NVDA",
		" aapl": "AAPL",
		"BRK.B": "BRK.B",
		"$tsla": "TSLA",
		"":      "",
	}
	for in, want := range cases {
		if got := normTicker(in); got != want {
			t.Errorf("normTicker(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNovelTickerSet(t *testing.T) {
	set := novelTickerSet([]string{"aapl,nvda"}, "tsla")
	for _, want := range []string{"AAPL", "NVDA", "TSLA"} {
		if !set[want] {
			t.Errorf("ticker set missing %q (%v)", want, set)
		}
	}
	if len(set) != 3 {
		t.Errorf("expected 3 tickers, got %d (%v)", len(set), set)
	}
	if len(novelTickerSet(nil, "")) != 0 {
		t.Errorf("empty inputs should yield empty (no-filter) set")
	}
}

func TestNovelNewsTickers(t *testing.T) {
	row := map[string]any{
		"stocks": []any{
			map[string]any{"name": "$TRX"},
			map[string]any{"name": "$BNB"},
			map[string]any{"name": ""},
		},
	}
	got := novelNewsTickers(row)
	// Empty names are skipped, so only TRX and BNB survive.
	if len(got) != 2 || got[0] != "TRX" || got[1] != "BNB" {
		t.Fatalf("novelNewsTickers = %v, want [TRX BNB]", got)
	}
}

func TestNovelEventTime(t *testing.T) {
	// numeric unix `updated`
	if got := novelEventTime(map[string]any{"updated": float64(1768937315)}); got.Unix() != 1768937315 {
		t.Errorf("unix updated parse = %v", got)
	}
	// RFC822 news `created`
	got := novelEventTime(map[string]any{"created": "Sun, 28 Jun 2026 23:15:47 -0400"})
	if got.IsZero() || got.Year() != 2026 || got.Month() != time.June {
		t.Errorf("RFC822 created parse = %v", got)
	}
	// date + time
	got = novelEventTime(map[string]any{"date": "2026-10-05", "time": "14:26:09"})
	if got.IsZero() || got.Year() != 2026 || got.Day() != 5 {
		t.Errorf("date+time parse = %v", got)
	}
	// nothing parseable
	if !novelEventTime(map[string]any{"foo": "bar"}).IsZero() {
		t.Errorf("expected zero time for unparseable row")
	}
}

func TestNovelFloat(t *testing.T) {
	if v, ok := novelFloat(map[string]any{"eps": "0.570"}, "eps"); !ok || v != 0.57 {
		t.Errorf("string float = %v,%v want 0.57,true", v, ok)
	}
	if _, ok := novelFloat(map[string]any{"eps": ""}, "eps"); ok {
		t.Errorf("empty string should be not-ok")
	}
	if v, ok := novelFloat(map[string]any{"n": float64(3)}, "n"); !ok || v != 3 {
		t.Errorf("numeric float = %v,%v", v, ok)
	}
	if _, ok := novelFloat(map[string]any{}, "missing"); ok {
		t.Errorf("missing key should be not-ok")
	}
}

func TestNovelNested(t *testing.T) {
	row := map[string]any{"security": map[string]any{"ticker": "INTC"}}
	if got := novelNested(row, "security", "ticker"); got != "INTC" {
		t.Errorf("novelNested = %q want INTC", got)
	}
	if got := novelNested(map[string]any{}, "security", "ticker"); got != "" {
		t.Errorf("missing parent should yield empty, got %q", got)
	}
}

func TestRound2(t *testing.T) {
	if got := round2(14.0049); got != 14.0 {
		t.Errorf("round2(14.0049) = %v want 14.0", got)
	}
	if got := round2(-0.3750); got != -0.38 {
		t.Errorf("round2(-0.3750) = %v want -0.38", got)
	}
}

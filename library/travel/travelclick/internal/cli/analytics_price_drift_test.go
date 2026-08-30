// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/store"
)

// TestNovelAnalyticsPriceDriftHelpWires smoke-tests that the analytics price-drift command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAnalyticsPriceDriftHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"analytics", "price-drift", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("analytics price-drift --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "price-drift"} {
		if !strings.Contains(help, want) {
			t.Fatalf("analytics price-drift --help missing %q in output:\n%s", want, help)
		}
	}
}

func driftSnapshot(hotel, room, plan, checkIn, checkOut, currency, capturedAt string, rate float64) store.RateSnapshot {
	return store.RateSnapshot{
		HotelID:      hotel,
		CheckIn:      checkIn,
		CheckOut:     checkOut,
		RoomTypeCode: room,
		RoomTypeName: room + "-name",
		RatePlanCode: plan,
		RatePlanName: plan + "-name",
		NightlyRate:  rate,
		Currency:     currency,
		CapturedAt:   capturedAt,
	}
}

func TestSelectPriceDriftSeriesPrefersLongestHistory(t *testing.T) {
	// Product A has prior history (100 then 120). Product B is new in the
	// same latest batch at 50. The last tied row must not win.
	snapshots := []store.RateSnapshot{
		driftSnapshot("102306", "A", "BAR", "2026-09-15", "2026-09-18", "USD", "2026-08-01T00:00:00Z", 100),
		driftSnapshot("102306", "A", "BAR", "2026-09-15", "2026-09-18", "USD", "2026-08-10T00:00:00Z", 120),
		driftSnapshot("102306", "B", "CHEAP", "2026-09-15", "2026-09-18", "USD", "2026-08-10T00:00:00Z", 50),
	}

	series := selectPriceDriftSeries(snapshots)
	if len(series) != 2 {
		t.Fatalf("expected product A history of 2, got %d: %+v", len(series), series)
	}
	if series[0].RoomTypeCode != "A" || series[0].RatePlanCode != "BAR" {
		t.Fatalf("expected product A, got room=%s plan=%s", series[0].RoomTypeCode, series[0].RatePlanCode)
	}
	if series[0].NightlyRate != 100 || series[1].NightlyRate != 120 {
		t.Fatalf("expected 100 then 120, got %f then %f", series[0].NightlyRate, series[1].NightlyRate)
	}
	drift := series[len(series)-1].NightlyRate - series[0].NightlyRate
	if drift != 20 {
		t.Fatalf("expected drift +20, got %f", drift)
	}
}

func TestSelectPriceDriftSeriesDeterministicAcrossShuffle(t *testing.T) {
	a1 := driftSnapshot("102306", "KING", "BAR", "2026-09-15", "2026-09-18", "USD", "2026-08-01T00:00:00Z", 100)
	a2 := driftSnapshot("102306", "KING", "BAR", "2026-09-15", "2026-09-18", "USD", "2026-08-10T00:00:00Z", 120)
	b1 := driftSnapshot("102306", "QUEEN", "PROMO", "2026-09-15", "2026-09-18", "USD", "2026-08-10T00:00:00Z", 50)

	orders := [][]store.RateSnapshot{
		{a1, a2, b1},
		{b1, a2, a1},
		{a2, b1, a1},
		{b1, a1, a2},
	}
	for i, snaps := range orders {
		series := selectPriceDriftSeries(snaps)
		if len(series) != 2 || series[0].RoomTypeCode != "KING" || series[1].NightlyRate != 120 {
			t.Fatalf("order %d: expected KING history 100/120, got %+v", i, series)
		}
	}
}

func TestSelectPriceDriftSeriesTieBreaksByDatesThenCurrency(t *testing.T) {
	// Same room/plan, same history length, same latest rate: only stay
	// dates or currency differ. Map iteration must not decide the winner.
	sep := []store.RateSnapshot{
		driftSnapshot("102306", "KING", "BAR", "2026-09-15", "2026-09-18", "USD", "2026-08-01T00:00:00Z", 100),
		driftSnapshot("102306", "KING", "BAR", "2026-09-15", "2026-09-18", "USD", "2026-08-10T00:00:00Z", 120),
	}
	oct := []store.RateSnapshot{
		driftSnapshot("102306", "KING", "BAR", "2026-10-01", "2026-10-04", "USD", "2026-08-01T00:00:00Z", 100),
		driftSnapshot("102306", "KING", "BAR", "2026-10-01", "2026-10-04", "USD", "2026-08-10T00:00:00Z", 120),
	}
	eur := []store.RateSnapshot{
		driftSnapshot("102306", "KING", "BAR", "2026-09-15", "2026-09-18", "EUR", "2026-08-01T00:00:00Z", 100),
		driftSnapshot("102306", "KING", "BAR", "2026-09-15", "2026-09-18", "EUR", "2026-08-10T00:00:00Z", 120),
	}

	dateOrders := [][]store.RateSnapshot{
		append(append([]store.RateSnapshot{}, sep...), oct...),
		append(append([]store.RateSnapshot{}, oct...), sep...),
	}
	for i, snaps := range dateOrders {
		series := selectPriceDriftSeries(snaps)
		if len(series) != 2 || series[0].CheckIn != "2026-09-15" || series[0].Currency != "USD" {
			t.Fatalf("date order %d: expected Sept USD series, got %+v", i, series)
		}
	}

	currencyOrders := [][]store.RateSnapshot{
		append(append([]store.RateSnapshot{}, eur...), sep...),
		append(append([]store.RateSnapshot{}, sep...), eur...),
	}
	for i, snaps := range currencyOrders {
		series := selectPriceDriftSeries(snaps)
		if len(series) != 2 || series[0].Currency != "EUR" || series[0].CheckIn != "2026-09-15" {
			t.Fatalf("currency order %d: expected EUR before USD, got %+v", i, series)
		}
	}
}

func TestAnalyticsPriceDriftCommandSelectsLongestHistory(t *testing.T) {
	testenv.Isolate(t, cliutil.DataDir)
	ctx := context.Background()
	dbPath := defaultDBPath("travelclick-pp-cli")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	// Insert B first so last-row / last-insert would pick the new product.
	for _, sn := range []store.RateSnapshot{
		driftSnapshot("102306", "B", "CHEAP", "2026-09-15", "2026-09-18", "USD", "2026-08-10T00:00:00Z", 50),
		driftSnapshot("102306", "A", "BAR", "2026-09-15", "2026-09-18", "USD", "2026-08-01T00:00:00Z", 100),
		driftSnapshot("102306", "A", "BAR", "2026-09-15", "2026-09-18", "USD", "2026-08-10T00:00:00Z", 120),
	} {
		row := sn
		if err := db.InsertRateSnapshot(ctx, &row); err != nil {
			t.Fatalf("insert snapshot: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	cmd := RootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"analytics", "price-drift", "--hotel", "102306", "--json", "--no-learn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("price-drift: %v (stderr=%q)", err, stderr.String())
	}

	var envelope struct {
		Data PriceDriftOutput `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal output: %v (stdout=%q)", err, stdout.String())
	}
	out := envelope.Data
	if out.HotelID == "" && out.LatestRate == 0 {
		if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
			t.Fatalf("unmarshal raw output: %v (stdout=%q)", err, stdout.String())
		}
	}
	if out.Currency != "USD" {
		t.Fatalf("expected USD, got %s (stdout=%q)", out.Currency, stdout.String())
	}
	if out.EarliestRate != 100 || out.LatestRate != 120 || out.Drift != 20 {
		t.Fatalf("expected A drift 100->120 (+20), got earliest=%f latest=%f drift=%f (stdout=%q)", out.EarliestRate, out.LatestRate, out.Drift, stdout.String())
	}
	if len(out.Timeline) != 2 || out.Timeline[1].RatePlanCode != "BAR" {
		t.Fatalf("expected BAR timeline, got %+v", out.Timeline)
	}
	if strings.Contains(stderr.String(), "only one snapshot") {
		t.Fatalf("did not expect single-snapshot warning: %q", stderr.String())
	}
}

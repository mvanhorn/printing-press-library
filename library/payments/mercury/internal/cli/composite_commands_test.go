// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestFlexFloatUnmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{`"-123.45"`, -123.45},
		{`67.89`, 67.89},
		{`"0"`, 0},
		{`""`, 0},
		{`null`, 0},
	}
	for _, tc := range cases {
		var f flexFloat
		if err := json.Unmarshal([]byte(tc.in), &f); err != nil {
			t.Fatalf("unmarshal %s: %v", tc.in, err)
		}
		if math.Abs(float64(f)-tc.want) > 1e-9 {
			t.Fatalf("flexFloat(%s) = %v, want %v", tc.in, float64(f), tc.want)
		}
	}
	var f flexFloat
	if err := json.Unmarshal([]byte(`"abc"`), &f); err == nil {
		t.Fatal("expected error for non-numeric string")
	}
}

// Confirms an amount delivered as a JSON number and as a quoted string both
// decode into the transaction struct.
func TestMercuryTxnDecodeAmountShapes(t *testing.T) {
	raw := `[
      {"id":"a","amount":-50.00,"postedAt":"2026-06-01T10:00:00Z","mercuryCategory":"software"},
      {"id":"b","amount":"-25.50","createdAt":"2026-06-02T10:00:00Z","mercuryCategory":"advertising"}
    ]`
	var txns []mercuryTxn
	if err := json.Unmarshal([]byte(raw), &txns); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(txns) != 2 {
		t.Fatalf("got %d txns, want 2", len(txns))
	}
	if float64(txns[0].Amount) != -50.0 || float64(txns[1].Amount) != -25.5 {
		t.Fatalf("amount decode wrong: %v / %v", float64(txns[0].Amount), float64(txns[1].Amount))
	}
}

func mkTxn(amount float64, posted string, category string) mercuryTxn {
	return mercuryTxn{Amount: flexFloat(amount), PostedAt: posted, MercuryCategory: category}
}

func TestSummarizeRunwayBurning(t *testing.T) {
	// Over 2 weeks: inflow 1000, outflow 5000 → net -4000 → weekly burn 2000.
	txns := []mercuryTxn{
		mkTxn(1000, "2026-06-01T00:00:00Z", ""),
		mkTxn(-3000, "2026-06-02T00:00:00Z", ""),
		mkTxn(-2000, "2026-06-03T00:00:00Z", ""),
	}
	s := summarizeRunway(20000, txns, 2)
	if s.Inflow != 1000 || s.Outflow != 5000 {
		t.Fatalf("flows wrong: in=%v out=%v", s.Inflow, s.Outflow)
	}
	if s.NetFlow != -4000 {
		t.Fatalf("net = %v, want -4000", s.NetFlow)
	}
	if s.AvgWeeklyBurn != 2000 {
		t.Fatalf("weekly burn = %v, want 2000", s.AvgWeeklyBurn)
	}
	if s.RunwayWeeks == nil || math.Abs(*s.RunwayWeeks-10.0) > 1e-9 {
		t.Fatalf("runway = %v, want 10.0", s.RunwayWeeks)
	}
	if s.CashFlowPositive {
		t.Fatal("should not be cash-flow positive while burning")
	}
}

func TestSummarizeRunwayCashFlowPositive(t *testing.T) {
	txns := []mercuryTxn{
		mkTxn(8000, "2026-06-01T00:00:00Z", ""),
		mkTxn(-2000, "2026-06-02T00:00:00Z", ""),
	}
	s := summarizeRunway(10000, txns, 4)
	if !s.CashFlowPositive {
		t.Fatalf("net %v over 4 weeks should be cash-flow positive", s.NetFlow)
	}
	if s.RunwayWeeks != nil {
		t.Fatalf("runway should be nil when cash-flow positive, got %v", *s.RunwayWeeks)
	}
}

func TestSummarizeByCategory(t *testing.T) {
	split := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	txns := []mercuryTxn{
		// current period (>= split)
		mkTxn(-300, "2026-06-04T00:00:00Z", "advertising"),
		mkTxn(-100, "2026-06-05T00:00:00Z", "software"),
		mkTxn(-50, "2026-06-05T00:00:00Z", ""), // uncategorized
		// prior period (< split)
		mkTxn(-200, "2026-06-01T00:00:00Z", "advertising"),
		// inflow ignored
		mkTxn(5000, "2026-06-04T00:00:00Z", "income"),
		// undated → dropped
		mkTxn(-999, "", "software"),
	}
	results, dropped := summarizeByCategory(txns, split)
	if dropped != 1 {
		t.Fatalf("dropped = %d, want 1", dropped)
	}
	byCat := map[string]categorySpend{}
	for _, r := range results {
		byCat[r.Category] = r
	}
	if byCat["advertising"].Current != 300 || byCat["advertising"].Prior != 200 || byCat["advertising"].Delta != 100 {
		t.Fatalf("advertising wrong: %+v", byCat["advertising"])
	}
	if byCat["software"].Current != 100 {
		t.Fatalf("software current = %v, want 100", byCat["software"].Current)
	}
	if byCat["uncategorized"].Current != 50 {
		t.Fatalf("uncategorized current = %v, want 50", byCat["uncategorized"].Current)
	}
	if _, ok := byCat["income"]; ok {
		t.Fatal("inflow category should not appear")
	}
	// Ranked by current desc: advertising (300) first.
	if results[0].Category != "advertising" {
		t.Fatalf("top category = %q, want advertising", results[0].Category)
	}
}

// Dry-run contract: both composites return before any network call and emit
// nothing on stdout.
func TestMercuryCompositesDryRunEmitNothing(t *testing.T) {
	for _, mk := range []struct {
		name string
		cmd  func(*rootFlags) *bytes.Buffer
	}{
		{"cash-runway", func(f *rootFlags) *bytes.Buffer {
			c := newCashRunwayCmd(f)
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetErr(&out)
			c.SetArgs([]string{})
			_ = c.Execute()
			return &out
		}},
		{"spend-by-category", func(f *rootFlags) *bytes.Buffer {
			c := newSpendByCategoryCmd(f)
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetErr(&out)
			c.SetArgs([]string{})
			_ = c.Execute()
			return &out
		}},
	} {
		t.Run(mk.name, func(t *testing.T) {
			out := mk.cmd(&rootFlags{dryRun: true})
			if out.Len() != 0 {
				t.Fatalf("%s dry-run emitted %q, want nothing", mk.name, out.String())
			}
		})
	}
}

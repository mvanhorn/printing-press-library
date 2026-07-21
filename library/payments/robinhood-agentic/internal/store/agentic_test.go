// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestGuardPolicyEvaluateOrder(t *testing.T) {
	cases := []struct {
		name     string
		policy   GuardPolicy
		symbol   string
		notional float64
		daySpent float64
		wantErr  bool
	}{
		{"empty policy allows", GuardPolicy{}, "AAPL", 100000, 0, false},
		{"kill switch blocks", GuardPolicy{KillSwitch: true}, "AAPL", 1, 0, true},
		{"under per-order cap", GuardPolicy{MaxOrderNotional: 500}, "AAPL", 400, 0, false},
		{"over per-order cap", GuardPolicy{MaxOrderNotional: 500}, "AAPL", 600, 0, true},
		{"under daily cap", GuardPolicy{DailyCapNotional: 1000}, "AAPL", 400, 500, false},
		{"over daily cap", GuardPolicy{DailyCapNotional: 1000}, "AAPL", 400, 700, true},
		{"allowlist miss blocks", GuardPolicy{Allowlist: []string{"MSFT"}}, "AAPL", 1, 0, true},
		{"allowlist hit passes", GuardPolicy{Allowlist: []string{"AAPL"}}, "aapl", 1, 0, false},
		{"denylist blocks", GuardPolicy{Denylist: []string{"AAPL"}}, "AAPL", 1, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason := c.policy.EvaluateOrder(c.symbol, c.notional, c.daySpent)
			if (reason != "") != c.wantErr {
				t.Errorf("EvaluateOrder reason=%q, wantErr=%v", reason, c.wantErr)
			}
		})
	}
}

func TestParseNotionalDetail(t *testing.T) {
	if got := parseNotionalDetail("notional=250.50"); got != 250.50 {
		t.Errorf("parseNotionalDetail = %v, want 250.50", got)
	}
	if got := parseNotionalDetail("something;notional=99.00;else"); got != 99.00 {
		t.Errorf("parseNotionalDetail multi = %v, want 99.00", got)
	}
	if got := parseNotionalDetail("no notional here"); got != 0 {
		t.Errorf("parseNotionalDetail none = %v, want 0", got)
	}
}

func TestAgenticRoundTrips(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// portfolio snapshot round-trip
	pf := json.RawMessage(`{"total_value":"1000.00","equity_value":"800.00","cash":"200.00","buying_power":{"buying_power":"200.00"}}`)
	if err := st.RecordPortfolioSnapshot("RH1", pf); err != nil {
		t.Fatal(err)
	}
	snaps, err := st.PortfolioSnapshots("RH1", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].TotalValue != "1000.00" || snaps[0].BuyingPower != "200.00" {
		t.Fatalf("snapshot round-trip wrong: %#v", snaps)
	}

	// write journal round-trip + filter
	_ = st.RecordWrite(WriteJournalEntry{Tool: "place_equity_order", Action: "place", Symbol: "AAPL", Outcome: "placed", Detail: "notional=400.00"})
	_ = st.RecordWrite(WriteJournalEntry{Tool: "place_equity_order", Action: "place", Symbol: "TSLA", Outcome: "blocked_guard", Detail: "denylist"})
	placed, err := st.WriteJournal(WriteJournalFilter{Outcome: "placed"})
	if err != nil || len(placed) != 1 || placed[0].Symbol != "AAPL" {
		t.Fatalf("placed filter wrong: %v %#v", err, placed)
	}
	blocked, _ := st.WriteJournal(WriteJournalFilter{OutcomePfx: "blocked"})
	if len(blocked) != 1 || blocked[0].Symbol != "TSLA" {
		t.Fatalf("blocked filter wrong: %#v", blocked)
	}
	sum, _ := st.SumPlacedNotionalSince(time.Now().Add(-time.Hour))
	if sum != 400.00 {
		t.Errorf("SumPlacedNotionalSince = %v, want 400", sum)
	}

	// guard policy round-trip
	want := GuardPolicy{MaxOrderNotional: 500, DailyCapNotional: 2000, Denylist: []string{"TSLA"}}
	if err := st.SetGuardPolicy(want); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetGuardPolicy()
	if err != nil || got.MaxOrderNotional != 500 || got.DailyCapNotional != 2000 || len(got.Denylist) != 1 {
		t.Fatalf("guard policy round-trip: %v %#v", err, got)
	}

	// tool-surface round-trip
	_ = st.RecordToolSurface(json.RawMessage(`[{"name":"get_accounts"},{"name":"get_portfolio"}]`))
	surf, err := st.ToolSurfaceSnapshots(1)
	if err != nil || len(surf) != 1 || surf[0].ToolCount != 2 {
		t.Fatalf("tool surface round-trip: %v %#v", err, surf)
	}
}

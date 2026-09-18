// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/store"
	"github.com/spf13/cobra"
)

// TestNovelRedemptionsSuggestHelpWires smoke-tests that the redemptions
// suggest command resolves at runtime and renders useful --help output.
// Catches wiring regressions (missing AddCommand, panicking RunE on --help,
// etc.) before review. Keep this smoke test when adding behavior-specific
// cases.
func TestNovelRedemptionsSuggestHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"redemptions", "suggest", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("redemptions suggest --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "suggest"} {
		if !strings.Contains(help, want) {
			t.Fatalf("redemptions suggest --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestRankRedemptionSuggestions covers the pure aggregation/ranking logic
// with no I/O: frequency ordering, recency tie-breaking, and the
// empty-reward-name fallback label.
func TestRankRedemptionSuggestions(t *testing.T) {
	tests := []struct {
		name string
		rows []redemptionHistoryRow
		want []redemptionSuggestion
	}{
		{
			name: "empty input yields no suggestions",
			rows: nil,
			want: []redemptionSuggestion{},
		},
		{
			name: "ranks by times redeemed descending",
			rows: []redemptionHistoryRow{
				{RewardName: "Coffee Gift Card", State: "fulfilled", CreatedAt: "2026-01-01T00:00:00Z"},
				{RewardName: "Book Credit", State: "fulfilled", CreatedAt: "2026-02-01T00:00:00Z"},
				{RewardName: "Coffee Gift Card", State: "fulfilled", CreatedAt: "2026-03-01T00:00:00Z"},
				{RewardName: "Coffee Gift Card", State: "fulfilled", CreatedAt: "2026-04-01T00:00:00Z"},
			},
			want: []redemptionSuggestion{
				{RewardName: "Coffee Gift Card", TimesRedeemed: 3, LastRedeemedAt: "2026-04-01T00:00:00Z", LastState: "fulfilled"},
				{RewardName: "Book Credit", TimesRedeemed: 1, LastRedeemedAt: "2026-02-01T00:00:00Z", LastState: "fulfilled"},
			},
		},
		{
			name: "ties on times redeemed break by most recent",
			rows: []redemptionHistoryRow{
				{RewardName: "A", State: "fulfilled", CreatedAt: "2026-01-01T00:00:00Z"},
				{RewardName: "B", State: "fulfilled", CreatedAt: "2026-06-01T00:00:00Z"},
			},
			want: []redemptionSuggestion{
				{RewardName: "B", TimesRedeemed: 1, LastRedeemedAt: "2026-06-01T00:00:00Z", LastState: "fulfilled"},
				{RewardName: "A", TimesRedeemed: 1, LastRedeemedAt: "2026-01-01T00:00:00Z", LastState: "fulfilled"},
			},
		},
		{
			name: "blank reward name falls back to a labeled placeholder",
			rows: []redemptionHistoryRow{
				{RewardName: "", State: "fulfilled", CreatedAt: "2026-05-01T00:00:00Z"},
			},
			want: []redemptionSuggestion{
				{RewardName: "(unnamed reward)", TimesRedeemed: 1, LastRedeemedAt: "2026-05-01T00:00:00Z", LastState: "fulfilled"},
			},
		},
		{
			name: "tracks the latest state, not just the first seen",
			rows: []redemptionHistoryRow{
				{RewardName: "Gift Card", State: "fulfilled", CreatedAt: "2026-01-01T00:00:00Z"},
				{RewardName: "Gift Card", State: "delivered", CreatedAt: "2026-02-01T00:00:00Z"},
			},
			want: []redemptionSuggestion{
				{RewardName: "Gift Card", TimesRedeemed: 2, LastRedeemedAt: "2026-02-01T00:00:00Z", LastState: "delivered"},
			},
		},
		{
			name: "excludes pending, denied, and failed attempts from the count entirely",
			rows: []redemptionHistoryRow{
				{RewardName: "Movie Tickets", State: "pending", CreatedAt: "2026-01-01T00:00:00Z"},
				{RewardName: "Movie Tickets", State: "pending", CreatedAt: "2026-01-02T00:00:00Z"},
				{RewardName: "Movie Tickets", State: "fulfilled", CreatedAt: "2026-01-03T00:00:00Z"},
				{RewardName: "Spa Day", State: "denied", CreatedAt: "2026-02-01T00:00:00Z"},
			},
			want: []redemptionSuggestion{
				// Movie Tickets has 1 completed redemption despite 2 pending
				// attempts -- times_redeemed must reflect only the completed
				// one, not 3.
				{RewardName: "Movie Tickets", TimesRedeemed: 1, LastRedeemedAt: "2026-01-03T00:00:00Z", LastState: "fulfilled"},
				// Spa Day has zero completed redemptions (its only row is
				// "denied") and must not appear at all -- there is no
				// evidence the user ever successfully redeemed it.
			},
		},
		{
			name: "reward with only non-completed attempts produces no suggestion",
			rows: []redemptionHistoryRow{
				{RewardName: "Concert Tickets", State: "rejected", CreatedAt: "2026-01-01T00:00:00Z"},
				{RewardName: "Concert Tickets", State: "EXPIRED", CreatedAt: "2026-01-05T00:00:00Z"},
			},
			want: []redemptionSuggestion{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rankRedemptionSuggestions(tt.rows)
			if len(got) != len(tt.want) {
				t.Fatalf("rankRedemptionSuggestions() returned %d entries, want %d\ngot:  %+v\nwant: %+v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("entry %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestLooksLikeCompletedRedemption covers the state denylist directly:
// case-insensitivity, substring matching, and the empty/unknown-state
// default of "treat as completed" (see the function's doc comment for why
// unrecognized states are not excluded).
func TestLooksLikeCompletedRedemption(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		{"fulfilled", true},
		{"delivered", true},
		{"", true},                  // unknown/empty -- not excluded by design
		{"some_new_state_2027", true}, // unrecognized -- not excluded by design
		{"pending", false},
		{"PENDING", false}, // case-insensitive
		{"denied", false},
		{"rejected", false},
		{"failed", false},
		{"cancelled", false},
		{"canceled", false},
		{"expired", false},
		{"EXPIRED", false},
		{"request_pending_review", false}, // substring match
	}
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := looksLikeCompletedRedemption(tt.state); got != tt.want {
				t.Errorf("looksLikeCompletedRedemption(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

// TestNovelRedemptionsSuggestNegativeLimitRejected covers the --limit usage
// guard: a negative value must fail fast with a usage error before any
// client/database access, rather than silently disabling truncation (the
// help text documents only 0 as the unlimited sentinel).
func TestNovelRedemptionsSuggestNegativeLimitRejected(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"redemptions", "suggest", "--limit", "-1"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("redemptions suggest --limit -1 succeeded, want a usage error")
	}
	if !strings.Contains(err.Error(), "--limit must be zero or greater") {
		t.Fatalf("unexpected error for --limit -1: %v", err)
	}
}

// TestNovelRedemptionsSuggestRejectsDataSourceLive covers the --data-source
// compatibility guard: this command's core value (ranked redemption
// history) is inherently local-derived, so an explicit --data-source live
// request must be rejected clearly rather than silently served from local
// data anyway. Runs before any client/database access, so no mirror or
// credentials are needed for this test.
func TestNovelRedemptionsSuggestRejectsDataSourceLive(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"redemptions", "suggest", "--data-source", "live"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("redemptions suggest --data-source live succeeded, want a rejection error")
	}
	if !strings.Contains(err.Error(), "no live equivalent for this command") {
		t.Fatalf("unexpected error for --data-source live: %v", err)
	}
}

// newRedemptionsSuggestTestCmd returns a bare *cobra.Command with a real
// context set. A zero-value &cobra.Command{} has a nil c.ctx until cobra's
// own Execute()/ExecuteContext() sets one -- Command.Context() has no
// nil-guard (just `return c.ctx`), so passing that nil context.Context into
// database/sql's QueryContext machinery deadlocks (the goroutine panics
// inside db.conn() dereferencing the nil ctx, and the test's deferred
// db.Close() cleanup then blocks forever on the same mutex db.conn() never
// released before panicking) rather than failing fast. Discovered via this
// file's own tests hanging past `go test`'s default timeout.
func newRedemptionsSuggestTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "bonusly-pp-cli"}
	cmd.SetContext(context.Background())
	return cmd
}

// TestLatestBalanceHistorySnapshot covers the balance_history fallback
// query directly against a real temp SQLite store (reusing the
// newSyncHintTestStore helper from sync_hint_test.go): no table/no rows
// returns a blank recordedAt rather than an error, and a populated table
// returns the most recent row by recorded_at -- not insertion order.
func TestLatestBalanceHistorySnapshot(t *testing.T) {
	t.Run("no balance_history table yet returns no snapshot", func(t *testing.T) {
		db := newSyncHintTestStore(t)
		cmd := newRedemptionsSuggestTestCmd()
		earning, giving, recordedAt := latestBalanceHistorySnapshot(cmd, db)
		if recordedAt != "" || earning != nil || giving != nil {
			t.Fatalf("got (%v, %v, %q), want (nil, nil, \"\") when balance_history does not exist", earning, giving, recordedAt)
		}
	})

	t.Run("returns the most recent row by recorded_at, not insertion order", func(t *testing.T) {
		db := newSyncHintTestStore(t)
		if err := store.EnsureBonuslyBalanceHistory(context.Background(), db.DB()); err != nil {
			t.Fatalf("EnsureBonuslyBalanceHistory: %v", err)
		}
		// Insert the newer snapshot first to prove ORDER BY recorded_at
		// DESC drives the result, not insertion/rowid order.
		if _, err := db.DB().Exec(
			`INSERT INTO balance_history (recorded_at, giving_balance, earning_balance, monthly_budget) VALUES (?, ?, ?, ?)`,
			"2026-09-10 12:00:00", 100, 250, 500,
		); err != nil {
			t.Fatalf("seed newer snapshot: %v", err)
		}
		if _, err := db.DB().Exec(
			`INSERT INTO balance_history (recorded_at, giving_balance, earning_balance, monthly_budget) VALUES (?, ?, ?, ?)`,
			"2026-09-01 09:00:00", 80, 200, 500,
		); err != nil {
			t.Fatalf("seed older snapshot: %v", err)
		}

		cmd := newRedemptionsSuggestTestCmd()
		earning, giving, recordedAt := latestBalanceHistorySnapshot(cmd, db)
		// The sqlite driver round-trips a DATETIME-typed column through its
		// own time formatting (observed as RFC3339 rather than the literal
		// string bound in), so assert on the date component rather than an
		// exact string match -- what matters here is that the *newer* row
		// won, not the driver's serialization format.
		if recordedAt == "" || !strings.Contains(recordedAt, "2026-09-10") {
			t.Fatalf("recordedAt = %q, want it to reflect the newer 2026-09-10 snapshot", recordedAt)
		}
		if earning == nil || *earning != 250 {
			t.Fatalf("earning = %v, want 250", earning)
		}
		if giving == nil || *giving != 100 {
			t.Fatalf("giving = %v, want 100", giving)
		}
	})

	t.Run("nil db returns no snapshot rather than panicking", func(t *testing.T) {
		cmd := newRedemptionsSuggestTestCmd()
		earning, giving, recordedAt := latestBalanceHistorySnapshot(cmd, nil)
		if recordedAt != "" || earning != nil || giving != nil {
			t.Fatalf("got (%v, %v, %q), want (nil, nil, \"\") for a nil db", earning, giving, recordedAt)
		}
	})
}

// TestParseBonuslyBalanceFields covers both accepted response shapes
// (enveloped and bare) plus the not-found case that signals the caller to
// fall back.
func TestParseBonuslyBalanceFields(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantOK      bool
		wantEarning *int64
		wantGiving  *int64
	}{
		{
			name:        "enveloped response",
			raw:         `{"result":{"earning_balance":250,"giving_balance":100}}`,
			wantOK:      true,
			wantEarning: int64Ptr(250),
			wantGiving:  int64Ptr(100),
		},
		{
			name:        "bare unwrapped response",
			raw:         `{"earning_balance":42,"giving_balance":7}`,
			wantOK:      true,
			wantEarning: int64Ptr(42),
			wantGiving:  int64Ptr(7),
		},
		{
			name:   "unrecognized shape returns ok=false",
			raw:    `{"totally":"unexpected"}`,
			wantOK: false,
		},
		{
			name:   "invalid JSON returns ok=false",
			raw:    `not json`,
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			earning, giving, ok := parseBonuslyBalanceFields([]byte(tt.raw))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !int64PtrEqual(earning, tt.wantEarning) {
				t.Fatalf("earning = %v, want %v", earning, tt.wantEarning)
			}
			if !int64PtrEqual(giving, tt.wantGiving) {
				t.Fatalf("giving = %v, want %v", giving, tt.wantGiving)
			}
		})
	}
}

func int64Ptr(v int64) *int64 { return &v }

func int64PtrEqual(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

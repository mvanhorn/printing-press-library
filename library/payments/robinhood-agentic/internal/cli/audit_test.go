// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the audit command: wiring smoke test plus table-driven coverage of
// buildAuditFilter, the pure flag-to-filter mapping. Hermetic — no network, no store IO.

package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/store"
)

// TestNovelAuditHelpWires smoke-tests that the audit command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review.
func TestNovelAuditHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"audit", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "audit", "--since", "--denied", "--tool", "--placed"} {
		if !strings.Contains(help, want) {
			t.Fatalf("audit --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestBuildAuditFilter(t *testing.T) {
	since := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		denied bool
		placed bool
		tool   string
		want   store.WriteJournalFilter
	}{
		{
			name: "no flags: only the time window",
			want: store.WriteJournalFilter{Since: since},
		},
		{
			name:   "denied maps to the blocked outcome prefix",
			denied: true,
			want:   store.WriteJournalFilter{Since: since, OutcomePfx: "blocked"},
		},
		{
			name:   "placed maps to the exact placed outcome",
			placed: true,
			want:   store.WriteJournalFilter{Since: since, Outcome: "placed"},
		},
		{
			name: "tool narrows to one tool",
			tool: "place_equity_order",
			want: store.WriteJournalFilter{Since: since, Tool: "place_equity_order"},
		},
		{
			name: "empty tool leaves the tool filter unset",
			tool: "",
			want: store.WriteJournalFilter{Since: since},
		},
		{
			name:   "denied combines with tool",
			denied: true,
			tool:   "cancel_equity_order",
			want:   store.WriteJournalFilter{Since: since, OutcomePfx: "blocked", Tool: "cancel_equity_order"},
		},
		{
			name:   "placed combines with tool",
			placed: true,
			tool:   "place_option_order",
			want:   store.WriteJournalFilter{Since: since, Outcome: "placed", Tool: "place_option_order"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAuditFilter(since, tt.denied, tt.placed, tt.tool)
			if !got.Since.Equal(tt.want.Since) {
				t.Errorf("Since = %v, want %v", got.Since, tt.want.Since)
			}
			if got.Outcome != tt.want.Outcome {
				t.Errorf("Outcome = %q, want %q", got.Outcome, tt.want.Outcome)
			}
			if got.OutcomePfx != tt.want.OutcomePfx {
				t.Errorf("OutcomePfx = %q, want %q", got.OutcomePfx, tt.want.OutcomePfx)
			}
			if got.Tool != tt.want.Tool {
				t.Errorf("Tool = %q, want %q", got.Tool, tt.want.Tool)
			}
		})
	}
}

func TestOrDash(t *testing.T) {
	if got := orDash(""); got != "-" {
		t.Errorf("orDash(\"\") = %q, want %q", got, "-")
	}
	if got := orDash("AAPL"); got != "AAPL" {
		t.Errorf("orDash(\"AAPL\") = %q, want %q", got, "AAPL")
	}
}

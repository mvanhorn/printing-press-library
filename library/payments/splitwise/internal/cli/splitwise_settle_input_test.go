// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestNameCommandsAcceptMultiWordPositionals guards the actual failure path: a
// multi-word name arrives as multiple positionals (the MCP command-mirror splits
// `args:"EDCO 2021"` into ["EDCO","2021"]). A command whose Args validator caps at
// one positional (cobra.ExactArgs(1)) rejects this BEFORE RunE runs, making the
// joinNameArgs rejoin unreachable. These name-positional commands must accept
// 2+ positionals so the rejoin can do its job.
func TestNameCommandsAcceptMultiWordPositionals(t *testing.T) {
	flags := &rootFlags{}
	cmds := map[string]*cobra.Command{
		"settle-up": newSettleUpCmd(flags),
		"resolve":   newResolveCmd(flags),
		"split":     newSplitCmd(flags),
		"ledger":    newLedgerCmd(flags),
	}
	for name, cmd := range cmds {
		if cmd.Args == nil {
			continue // ArbitraryArgs already accepts multiple positionals
		}
		if err := cmd.Args(cmd, []string{"EDCO", "2021"}); err != nil {
			t.Errorf("%s rejected a split multi-word name (2 positionals): %v", name, err)
		}
	}
}

// TestJoinNameArgs: a multi-word positional (a group/friend name) that
// arrives as multiple args — e.g. when the MCP command-mirror whitespace-splits
// `args:"EDCO 2021"` into ["EDCO","2021"] — must be rejoined into the full name,
// not truncated to the first token.
func TestJoinNameArgs(t *testing.T) {
	if got := joinNameArgs([]string{"EDCO", "2021"}); got != "EDCO 2021" {
		t.Fatalf("joinNameArgs([EDCO 2021]) = %q, want \"EDCO 2021\"", got)
	}
	if got := joinNameArgs([]string{"  Tahoe Trip  "}); got != "Tahoe Trip" {
		t.Fatalf("joinNameArgs trims, got %q", got)
	}
	if got := joinNameArgs([]string{"28194161"}); got != "28194161" {
		t.Fatalf("joinNameArgs single numeric, got %q", got)
	}
}

// TestSettleResolvesMultiWordGroupName is the regression for the dogfood finding:
// the exact group name "EDCO 2021" arriving as split args must resolve to the one
// EDCO 2021 group, NOT error as the ambiguous prefix "EDCO" (which substring-matches
// both EDCO 2021 and EDCO 2022).
func TestSettleResolvesMultiWordGroupName(t *testing.T) {
	groups := []Group{{ID: 1, Name: "EDCO 2021"}, {ID: 2, Name: "EDCO 2022"}}
	input := joinNameArgs([]string{"EDCO", "2021"})
	g, ok, err := resolveSettleGroup(input, groups)
	if err != nil {
		t.Fatalf("unexpected ambiguity error: %v", err)
	}
	if !ok || g.ID != 1 {
		t.Fatalf("expected unique EDCO 2021 (id 1), got ok=%v id=%d", ok, g.ID)
	}
}

// TestLedgerResolvesMultiWordGroupName is the parallel regression for the ledger
// and split commands, which resolve group names via resolveLedgerGroup /
// resolveSettleGroup. The exact group name "EDCO 2021" arriving as split args must
// resolve to the one EDCO 2021 group, not error as the ambiguous prefix "EDCO".
func TestLedgerResolvesMultiWordGroupName(t *testing.T) {
	groups := []Group{{ID: 1, Name: "EDCO 2021"}, {ID: 2, Name: "EDCO 2022"}}
	input := joinNameArgs([]string{"EDCO", "2021"})
	id, name, _, err := resolveLedgerGroup(input, groups)
	if err != nil {
		t.Fatalf("unexpected ambiguity error: %v", err)
	}
	if id != 1 || name != "EDCO 2021" {
		t.Fatalf("expected unique EDCO 2021 (id 1), got id=%d name=%q", id, name)
	}
}

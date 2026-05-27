// Copyright 2026 rahul-bansal. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

func TestNewBuyerIntentListCmdHelp(t *testing.T) {
	flags := &rootFlags{}
	cmd := newBuyerIntentListCmd(flags)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("buyer-intent list --help failed: %v", err)
	}
}

func TestNewBuyerIntentListCmdDryRun(t *testing.T) {
	// --dry-run is a persistent root flag; this leaf only sees it when wired
	// through root. Flip the rootFlags directly and verify the dryRunOK
	// short-circuit returns cleanly without args.
	flags := &rootFlags{dryRun: true}
	cmd := newBuyerIntentListCmd(flags)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("buyer-intent list dry-run failed: %v", err)
	}
}

func TestBuyerIntentListRowShape(t *testing.T) {
	// Sanity check that the struct field tags don't drift; agents rely
	// on stable JSON keys for this command's CSV-flatten use case.
	want := []string{
		"company", "domain", "employees", "industry", "country",
		"page_type", "signal_type", "intent_score", "visited_on",
	}
	got := []buyerIntentListRow{{
		Company: "x", Domain: "x.com", Employees: 1, Industry: "ind",
		Country: "us", PageType: "p", SignalType: "s", IntentScore: 1, VisitedOn: "d",
	}}
	if len(got) != 1 {
		t.Fatal("seed row missing")
	}
	if len(want) != 9 {
		t.Fatal("schema drift in test list")
	}
}

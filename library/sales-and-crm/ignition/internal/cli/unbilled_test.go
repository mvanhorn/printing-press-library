// Copyright 2026 corben-tech and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestNovelUnbilledHelpWires smoke-tests that the unbilled command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelUnbilledHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"unbilled", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unbilled --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "unbilled", "agreed work not yet invoiced (BILLING_ITEM status UNBILLED)", "invoicing to-do list"} {
		if !strings.Contains(help, want) {
			t.Fatalf("unbilled --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestBuildUnbilledViewGroupsOnlyUnbilledBillingItems(t *testing.T) {
	nodes := []searchNode{
		{
			ID: "bill_1", TypeName: "BillingItemResult", BillingItemStatus: "UNBILLED",
			Amount: &searchMoney{Format: "$1,200.50"}, AmountWithTax: &searchMoney{Format: "$9,999.00"},
			ServiceName: "Tax return", Client: &searchClientRef{ID: "cli_2", Name: "Beta"},
		},
		{
			ID: "bill_2", TypeName: "BillingItemResult", BillingItemStatus: "unbilled",
			Amount: &searchMoney{Format: "$300.00"}, ServiceName: "Advisory",
			Client: &searchClientRef{ID: "cli_1", Name: "Acme"},
		},
		{
			ID: "bill_3", TypeName: "BillingItemResult", BillingItemStatus: " UnBiLlEd ",
			Amount: &searchMoney{Format: "$50.25"}, ServiceName: "Bookkeeping",
			Client: &searchClientRef{ID: "cli_1", Name: "Acme"},
		},
		{
			ID: "bill_4", TypeName: "BillingItemResult", BillingItemStatus: "BILLED",
			Amount: &searchMoney{Format: "$5,000.00"}, ServiceName: "Already invoiced",
			Client: &searchClientRef{ID: "cli_1", Name: "Acme"},
		},
	}

	view := buildUnbilledView(nodes)
	if view.Scanned != 4 || view.ItemCount != 3 || len(view.Unbilled) != 2 {
		t.Fatalf("unexpected unbilled counts: %#v", view)
	}
	if view.Unbilled[0].ClientName != "Acme" || view.Unbilled[0].ItemCount != 2 {
		t.Fatalf("unexpected Acme row: %#v", view.Unbilled[0])
	}
	assertFloat(t, view.Unbilled[0].Amount, 350.25)
	assertFloat(t, view.GrandTotal, 1550.75)
	if got := strings.Join(view.Unbilled[0].Services, ","); got != "Advisory,Bookkeeping" {
		t.Fatalf("Acme services = %q, want Advisory,Bookkeeping", got)
	}

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"unbilled", "grand_total", "item_count", "scanned"} {
		if _, ok := envelope[key]; !ok {
			t.Fatalf("unbilled JSON missing %q: %s", key, raw)
		}
	}
}

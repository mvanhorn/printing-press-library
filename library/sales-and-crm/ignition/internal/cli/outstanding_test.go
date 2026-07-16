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

// TestNovelOutstandingHelpWires smoke-tests that the outstanding command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelOutstandingHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"outstanding", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("outstanding --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "outstanding", "genuine open/failed accounts receivable (unpaid invoices), not scheduled billing"} {
		if !strings.Contains(help, want) {
			t.Fatalf("outstanding --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestBuildOutstandingViewCountsOnlyOpenOrFailedInvoices(t *testing.T) {
	nodes := []searchNode{
		{
			ID: "inv_requested", TypeName: "InvoiceResult",
			AmountWithTax:   &searchMoney{Format: "$100.00"},
			PaymentProgress: &searchPaymentProgress{Status: "PAYMENT_REQUESTED"},
			Client:          &searchClientRef{ID: "cli_2", Name: "Beta"},
		},
		{
			ID: "inv_failed", TypeName: "InvoiceResult",
			AmountWithTax:   &searchMoney{Format: "$250.50"},
			PaymentProgress: &searchPaymentProgress{Status: "collection_failed"},
			Client:          &searchClientRef{ID: "cli_1", Name: "Acme"},
		},
		{
			ID: "inv_overdue", TypeName: "InvoiceResult",
			AmountWithTax: &searchMoney{Format: "$25.00"},
			PaymentStatus: &searchPaymentStatus{DisplayName: "Overdue"},
			Client:        &searchClientRef{ID: "cli_1", Name: "Acme"},
		},
		{
			ID: "inv_paid", TypeName: "InvoiceResult",
			AmountWithTax:   &searchMoney{Format: "$999.00"},
			PaymentProgress: &searchPaymentProgress{Status: "PAID_OUT"},
			Client:          &searchClientRef{ID: "cli_1", Name: "Acme"},
		},
		{
			ID: "inv_refunded", TypeName: "InvoiceResult",
			AmountWithTax:   &searchMoney{Format: "$999.00"},
			PaymentProgress: &searchPaymentProgress{Status: "refunded"},
			Client:          &searchClientRef{ID: "cli_1", Name: "Acme"},
		},
		{
			ID: "inv_canceled", TypeName: "InvoiceResult",
			AmountWithTax: &searchMoney{Format: "$999.00"},
			PaymentStatus: &searchPaymentStatus{DisplayName: "Canceled"},
			Client:        &searchClientRef{ID: "cli_1", Name: "Acme"},
		},
		{
			ID: "inv_lost", TypeName: "InvoiceResult",
			AmountWithTax: &searchMoney{Format: "$999.00"},
			PaymentStatus: &searchPaymentStatus{DisplayName: "Dispute Lost"},
			Client:        &searchClientRef{ID: "cli_1", Name: "Acme"},
		},
		{
			ID: "inv_empty", TypeName: "InvoiceResult",
			AmountWithTax: &searchMoney{Format: "$999.00"},
			Client:        &searchClientRef{ID: "cli_1", Name: "Acme"},
		},
		{
			ID: "bill_unbilled", TypeName: "BillingItemResult",
			Amount: &searchMoney{Format: "$5,000.00"}, BillingItemStatus: "UNBILLED",
			Client: &searchClientRef{ID: "cli_1", Name: "Acme"},
		},
	}

	view := buildOutstandingView(nodes)
	if view.Scanned != len(nodes) || view.InvoiceCount != 3 || len(view.Clients) != 2 {
		t.Fatalf("unexpected outstanding counts: %#v", view)
	}
	if view.Clients[0].ClientName != "Acme" || view.Clients[0].ItemCount != 2 {
		t.Fatalf("unexpected Acme row: %#v", view.Clients[0])
	}
	assertFloat(t, view.Clients[0].OutstandingTotal, 275.50)
	assertFloat(t, view.GrandTotal, 375.50)

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"clients", "grand_total", "invoice_count", "scanned"} {
		if _, ok := envelope[key]; !ok {
			t.Fatalf("outstanding JSON missing %q: %s", key, raw)
		}
	}
}

func TestOutstandingStatusUsesVerifiedTerminalStates(t *testing.T) {
	for _, status := range []string{"", "PAID_OUT", "paid out", "REFUNDED", "canceled", "dispute-lost"} {
		if isOutstandingStatus(status) {
			t.Errorf("isOutstandingStatus(%q) = true, want false", status)
		}
	}
	for _, status := range []string{"PAYMENT_REQUESTED", "collection_failed", "Overdue"} {
		if !isOutstandingStatus(status) {
			t.Errorf("isOutstandingStatus(%q) = false, want true", status)
		}
	}
}

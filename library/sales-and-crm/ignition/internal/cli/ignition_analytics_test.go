// Copyright 2026 corben-tech and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

const (
	verifiedInvoiceSelection = `... on InvoiceResult { id externalNumber ledgerName billedOn collectionOn amountWithTax { format } paymentStatus { displayName } paymentProgress { status } client { id name } __typename }`
	verifiedBillingSelection = `... on BillingItemResult { id amount { format } amountWithTax { format } billingItemStatus billingStrategy client { id name } date itemPrice { description displayName type } serviceName __typename }`
)

func TestSearchQueriesUseVerifiedInvoiceAndBillingFields(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		selection string
	}{
		{name: "analytics invoice", query: invoiceSearchQuery, selection: verifiedInvoiceSelection},
		{name: "analytics billing", query: billingSearchQuery, selection: verifiedBillingSelection},
		{name: "invoice flag default", query: newSearchIndexInvoicesCmd(&rootFlags{}).Flags().Lookup("query").DefValue, selection: verifiedInvoiceSelection},
		{name: "billing flag default", query: newSearchIndexBillingItemsCmd(&rootFlags{}).Flags().Lookup("query").DefValue, selection: verifiedBillingSelection},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(normalizeWhitespace(tt.query), normalizeWhitespace(tt.selection)) {
				t.Fatalf("query does not contain verified selection:\n%s", tt.query)
			}
			for _, guessed := range []string{" number ", " total ", "... on BillingItemResult { id name status"} {
				if strings.Contains(" "+normalizeWhitespace(tt.query)+" ", guessed) {
					t.Fatalf("query still contains guessed field pattern %q:\n%s", guessed, tt.query)
				}
			}
		})
	}
}

func TestSearchQueryDefaultsKeepVerifiedPagination(t *testing.T) {
	for _, variables := range []string{
		newSearchIndexInvoicesCmd(&rootFlags{}).Flags().Lookup("variables").DefValue,
		newSearchIndexBillingItemsCmd(&rootFlags{}).Flags().Lookup("variables").DefValue,
	} {
		if !strings.Contains(variables, `"pageNumber":1`) || !strings.Contains(variables, `"pageSize":200`) {
			t.Fatalf("pagination default changed: %s", variables)
		}
	}
}

func TestParseMoneyFormat(t *testing.T) {
	tests := map[string]float64{
		"$1,234.56": 1234.56,
		"-$50.00":   -50,
		"USD 75":    75,
		"":          0,
	}
	for input, want := range tests {
		if got := parseMoneyFormat(input); got != want {
			t.Errorf("parseMoneyFormat(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestAnalyticsViewsUseVerifiedInvoiceAndBillingFields(t *testing.T) {
	failedInvoice := mustDecodeSearchNode(t, `{
		"id":"inv_internal","externalNumber":"INV-42","ledgerName":"General",
		"amountWithTax":{"format":"$1,234.56"},
		"paymentStatus":{"displayName":"Overdue"},
		"paymentProgress":{"status":"COLLECTION_FAILED"},
		"client":{"id":"cli_1","name":"Acme"},"__typename":"InvoiceResult"
	}`)
	paidInvoice := mustDecodeSearchNode(t, `{
		"id":"inv_paid","externalNumber":"INV-43",
		"amountWithTax":{"format":"$100.00"},
		"paymentStatus":{"displayName":"Paid"},
		"paymentProgress":{"status":"PAID_OUT"},
		"client":{"id":"cli_1","name":"Acme"},"__typename":"InvoiceResult"
	}`)
	declinedBilling := mustDecodeSearchNode(t, `{
		"id":"bill_1","amount":{"format":"-$50.00"},"amountWithTax":{"format":"-$55.00"},
		"billingItemStatus":"UNBILLED","billingStrategy":"ON_ACCEPTANCE",
		"client":{"id":"cli_1","name":"Acme"},"date":"2026-07-09",
		"itemPrice":{"description":"Advisory","displayName":"Advisory","type":"FIXED"},
		"serviceName":"Tax advisory","__typename":"BillingItemResult"
	}`)

	if got := nodeAmount(failedInvoice); got != 1234.56 {
		t.Fatalf("invoice amount = %v, want 1234.56", got)
	}
	if got := nodeStatus(failedInvoice); got != "COLLECTION_FAILED" {
		t.Fatalf("invoice status = %q, want COLLECTION_FAILED", got)
	}
	if got := nodeDisplayID(failedInvoice); got != "INV-42" {
		t.Fatalf("invoice display ID = %q, want INV-42", got)
	}
	if got := nodeAmount(declinedBilling); got != -50 {
		t.Fatalf("billing amount = %v, want -50 from amount.format", got)
	}
	if declinedBilling.ServiceName != "Tax advisory" {
		t.Fatalf("billing service name = %q, want Tax advisory", declinedBilling.ServiceName)
	}

	outstanding := buildOutstandingView([]searchNode{failedInvoice, paidInvoice, declinedBilling})
	if outstanding.Scanned != 3 || outstanding.InvoiceCount != 1 || len(outstanding.Clients) != 1 || outstanding.Clients[0].ItemCount != 1 {
		t.Fatalf("unexpected outstanding view: %#v", outstanding)
	}
	assertFloat(t, outstanding.GrandTotal, 1234.56)

	clientBilling := buildClientBillingView("cli_1", nil, []searchNode{failedInvoice, paidInvoice}, []searchNode{declinedBilling})
	assertFloat(t, clientBilling.InvoicedTotal, 1334.56)
	assertFloat(t, clientBilling.OutstandingTotal, 1234.56)

	rejected := buildRejectedPaymentsView([]searchNode{failedInvoice, paidInvoice, declinedBilling})
	if rejected.Count != 1 || len(rejected.Clients) != 1 {
		t.Fatalf("unexpected rejected-payments view: %#v", rejected)
	}
	if got := rejected.Clients[0].Items[0].ID; got != "INV-42" {
		t.Fatalf("rejected invoice ID = %q, want external number INV-42", got)
	}
}

func TestProposalPipelineAnalyticsRemainUnchanged(t *testing.T) {
	accepted := mustDecodeSearchNode(t, `{"id":"prop_1","name":"Annual tax","status":"ACCEPTED","client":{"id":"cli_1","name":"Acme"},"__typename":"ProposalResult"}`)
	draft := mustDecodeSearchNode(t, `{"id":"prop_2","name":"Planning","status":"DRAFT","client":{"id":"cli_2","name":"Beta"},"__typename":"ProposalResult"}`)
	pipeline := buildPipelineView([]searchNode{accepted, draft})
	if pipeline.ByStatus["ACCEPTED"] != 1 || pipeline.ByStatus["DRAFT"] != 1 {
		t.Fatalf("unexpected pipeline view: %#v", pipeline)
	}
}

func TestInvoiceStatusFallsBackToPaymentStatus(t *testing.T) {
	node := mustDecodeSearchNode(t, `{"id":"inv_1","paymentStatus":{"displayName":"Overdue"},"paymentProgress":{"status":""},"__typename":"InvoiceResult"}`)
	if got := nodeStatus(node); got != "Overdue" {
		t.Fatalf("nodeStatus() = %q, want Overdue", got)
	}
	if isOutstandingStatus("") {
		t.Fatal("empty status must not be treated as outstanding")
	}
}

func mustDecodeSearchNode(t *testing.T, input string) searchNode {
	t.Helper()
	node, err := decodeSearchNode(json.RawMessage(input))
	if err != nil {
		t.Fatalf("decodeSearchNode() error = %v", err)
	}
	return node
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func assertFloat(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeEndpointTable(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		bad  bool
	}{
		{name: "ideas short name", in: "ideas", want: EndpointIdeas},
		{name: "ideas RPC", in: "customers.generateKeywordIdeas", want: EndpointIdeas},
		{name: "historical short name", in: "historical", want: EndpointHistorical},
		{name: "historical RPC", in: "customers.generateKeywordHistoricalMetrics", want: EndpointHistorical},
		{name: "account helper", in: "customers.search", want: EndpointAccount},
		{name: "list accessible", in: "listAccessibleCustomers", want: EndpointAccount},
		{name: "empty", in: "", bad: true},
		{name: "unsupported", in: "campaigns.list", bad: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeEndpoint(test.in)
			if test.bad {
				if err == nil || !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("NormalizeEndpoint(%q) error = %v, want invalid input", test.in, err)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("NormalizeEndpoint(%q) = %q, %v; want %q", test.in, got, err, test.want)
			}
		})
	}
}

func TestParseYearMonthAndVendorMonthTable(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Month
		bad  bool
	}{
		{name: "first month", in: "2020-01", want: Month{Year: 2020, Number: 1}},
		{name: "last month", in: "2020-12", want: Month{Year: 2020, Number: 12}},
		{name: "day rejected", in: "2020-01-01", bad: true},
		{name: "zero month rejected", in: "2020-00", bad: true},
		{name: "empty rejected", in: "", bad: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseYearMonth(test.in)
			if test.bad {
				if err == nil || !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("ParseYearMonth(%q) error = %v, want invalid input", test.in, err)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("ParseYearMonth(%q) = %#v, %v; want %#v", test.in, got, err, test.want)
			}
		})
	}

	names := []string{"JANUARY", "FEBRUARY", "MARCH", "APRIL", "MAY", "JUNE", "JULY", "AUGUST", "SEPTEMBER", "OCTOBER", "NOVEMBER", "DECEMBER"}
	for index, name := range names {
		raw := json.RawMessage(`"` + name + `"`)
		month, ok, flag := parseVendorMonth(raw)
		if !ok || flag != "" || month.Number != index+1 {
			t.Errorf("vendor month %s = %#v, ok=%v flag=%q; want number %d", name, month, ok, flag, index+1)
		}
	}
	for _, test := range []struct {
		raw  string
		want int
	}{
		{raw: "2", want: 1},   // legacy protobuf January
		{raw: "13", want: 12}, // legacy protobuf December
	} {
		month, ok, flag := parseVendorMonth(json.RawMessage(test.raw))
		if !ok || flag != "" || month.Number != test.want {
			t.Errorf("legacy month %s = %#v, ok=%v flag=%q; want number %d", test.raw, month, ok, flag, test.want)
		}
	}
	for _, raw := range []string{"1", "14", `"NOT_A_MONTH"`, "null"} {
		if _, ok, flag := parseVendorMonth(json.RawMessage(raw)); ok || flag == "" {
			t.Errorf("parseVendorMonth(%s) = ok=%v flag=%q; want invalid", raw, ok, flag)
		}
	}
}

func TestPortfolioLifecyclePreservesEvidenceAndRows(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	input := SnapshotInput{
		RunID:              "run-one",
		Endpoint:           EndpointHistorical,
		APIVersion:         "v25",
		DiscoveryRevision:  "discovery-rev",
		CustomerID:         "1234567890",
		LoginCustomerID:    "0987654321",
		Language:           "languageConstants/1000",
		Network:            NetworkGoogleSearch,
		GeoTargetConstants: []string{"geoTargetConstants/2840", "geoTargetConstants/2124"},
		SubmittedSeeds:     []string{"alpha seed", "beta seed"},
		SubmittedKeywords:  []string{"alpha", "ALPHA"},
		RequestedStart:     "2020-01",
		RequestedEnd:       "2020-03",
		RequestBody:        json.RawMessage(`{"keywords":["alpha","ALPHA"]}`),
		SourceVariant:      "keyword_mode",
		Metadata:           json.RawMessage(`{"fixture":true}`),
	}
	fetchedAt := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	snapshot, err := store.StartSnapshot(ctx, input, fetchedAt)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := store.StartSnapshot(ctx, input, fetchedAt)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ID == repeated.ID {
		t.Fatal("repeated snapshots must have distinct IDs")
	}
	if got := strings.Join(snapshot.GeoTargetConstants, ","); got != "geoTargetConstants/2124,geoTargetConstants/2840" {
		t.Fatalf("geo set order = %q", got)
	}
	if snapshot.RequestBodyHash != bytesHash([]byte(`{"keywords":["alpha","ALPHA"]}`)) {
		t.Fatalf("snapshot request hash = %q", snapshot.RequestBodyHash)
	}

	accountIDs, err := store.AppendReceipt(ctx, ReceiptInput{
		SnapshotID:  snapshot.ID,
		Endpoint:    EndpointAccount,
		PageNumber:  0,
		BatchNumber: 0,
		Attempt:     0,
		RequestBody: json.RawMessage(`{"query":"currency"}`),
		HTTPStatus:  200,
		Body:        []byte(`{"results":[],"account":{"currencyCode":"EUR"}}`),
		FetchedAt:   fetchedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NormalizeReceipt(ctx, accountIDs.ReceiptID); err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("account helper normalization = %v; want raw-only invalid input", err)
	}
	if err := store.SetAccountCurrency(ctx, snapshot.ID, "eur", "live_GAQL", accountIDs.ReceiptID); err != nil {
		t.Fatal(err)
	}

	body := []byte(`{
  "unknownFutureField": {"preserve": true},
  "results": [{
    "text": "alpha",
    "keywordMetrics": {
      "competition": "UNSPECIFIED",
      "avgMonthlySearches": "0",
      "lowTopOfPageBidMicros": "9007199254740993",
      "highTopOfPageBidMicros": null,
      "monthlySearchVolumes": [
        {"month": "JANUARY", "year": "2020", "monthlySearches": "17"},
        {"month": 3, "year": "2020", "monthlySearches": null},
        {"month": "MARCH", "year": "2020", "monthlySearches": "0"},
        {"month": "SEPTEMBER", "year": "2099", "monthlySearches": "5"}
      ]
    }
  }]
}`)
	plannerIDs, err := store.AppendReceipt(ctx, ReceiptInput{
		SnapshotID:        snapshot.ID,
		Endpoint:          EndpointHistorical,
		PageNumber:        0,
		BatchNumber:       0,
		Attempt:           0,
		RequestBody:       json.RawMessage(`{"keywords":["alpha","ALPHA"]}`),
		SubmittedKeywords: []string{"alpha", "ALPHA"},
		HTTPStatus:        200,
		GoogleRequestID:   "req-fixture-1",
		Body:              body,
		FetchedAt:         fetchedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plannerIDs.BodySHA256 != bytesHash(body) {
		t.Fatalf("receipt hash = %q; want %q", plannerIDs.BodySHA256, bytesHash(body))
	}
	result, err := store.NormalizeReceipt(ctx, plannerIDs.ReceiptID)
	if err != nil {
		t.Fatal(err)
	}
	if result.KeywordRows != 1 || result.MonthlyRows != 3 || result.VariantRows != 2 || result.CoverageRows != 1 || !result.Complete {
		t.Fatalf("normalization result = %#v", result)
	}
	if err := store.Finish(ctx, FinishInput{SnapshotID: snapshot.ID, Complete: true}); err != nil {
		t.Fatal(err)
	}

	rows, err := store.Rows(ctx, QueryOptions{SnapshotID: snapshot.ID, IncludeIncomplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("Rows returned %d; want three closed monthly observations", len(rows))
	}
	if rows[0].Month != "2020-01-01" || rows[1].Month != "2020-02-01" || rows[2].Month != "2020-03-01" {
		t.Fatalf("row months = %q, %q, %q", rows[0].Month, rows[1].Month, rows[2].Month)
	}
	if rows[0].MonthlySearches == nil || *rows[0].MonthlySearches != 17 {
		t.Fatalf("January value = %#v", rows[0].MonthlySearches)
	}
	if rows[1].MonthlySearches != nil {
		t.Fatalf("February null became %#v", rows[1].MonthlySearches)
	}
	if rows[2].MonthlySearches == nil || *rows[2].MonthlySearches != 0 {
		t.Fatalf("March zero became %#v", rows[2].MonthlySearches)
	}
	if rows[0].AverageMonthlySearches == nil || *rows[0].AverageMonthlySearches != 0 {
		t.Fatalf("large int64 was not preserved: %#v", rows[0].AverageMonthlySearches)
	}
	if rows[0].LowBidMicros == nil || *rows[0].LowBidMicros != 9007199254740993 {
		t.Fatalf("large bid was not preserved: %#v", rows[0].LowBidMicros)
	}
	if rows[0].HighBidMicros != nil || rows[0].AverageCPCMicros != nil {
		t.Fatalf("null or absent optional money field changed: high=%#v cpc=%#v", rows[0].HighBidMicros, rows[0].AverageCPCMicros)
	}
	if rows[0].Currency != "EUR" || rows[0].CurrencySource != "live_GAQL" {
		t.Fatalf("currency provenance = %q/%q", rows[0].Currency, rows[0].CurrencySource)
	}
	if !contains(rows[0].Flags, "ambiguous_zero_unspecified") {
		t.Fatalf("metric warning was not propagated to monthly row: %#v", rows[0].Flags)
	}
	var physicalMonthlyRows int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM monthly_volumes WHERE snapshot_id = ?`, snapshot.ID).Scan(&physicalMonthlyRows); err != nil {
		t.Fatal(err)
	}
	if physicalMonthlyRows != 3 {
		t.Fatalf("raw-only future month entered normalized table: %d physical rows", physicalMonthlyRows)
	}

	available, err := store.Rows(ctx, QueryOptions{SnapshotID: snapshot.ID, IncludeIncomplete: true, AvailableOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(available) != 2 {
		t.Fatalf("AvailableOnly returned %d; want two values", len(available))
	}
	endMonth, err := store.Rows(ctx, QueryOptions{SnapshotID: snapshot.ID, StartMonth: "2020-02", EndMonth: "2020-02", IncludeIncomplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(endMonth) != 1 || endMonth[0].Month != "2020-02-01" {
		t.Fatalf("end-month filter = %#v", endMonth)
	}
	if got, err := store.Search(ctx, "alpha", QueryOptions{SnapshotID: snapshot.ID}); err != nil || len(got) != 3 {
		t.Fatalf("Search(alpha) = %d, %v; want three rows", len(got), err)
	}
	if got, err := store.Search(ctx, "nomatch%", QueryOptions{SnapshotID: snapshot.ID}); err != nil || len(got) != 0 {
		t.Fatalf("literal wildcard search = %d, %v; want empty", len(got), err)
	}

	view, err := store.ShowSnapshot(ctx, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Snapshot.Finalized || view.Snapshot.Status != StatusComplete || len(view.Receipts) != 2 || len(view.Rows) != 3 || len(view.Coverage) != 1 {
		t.Fatalf("snapshot view lifecycle = status %q finalized=%v receipts=%d rows=%d coverage=%d", view.Snapshot.Status, view.Snapshot.Finalized, len(view.Receipts), len(view.Rows), len(view.Coverage))
	}
	var plannerReceipt ReceiptView
	for _, receipt := range view.Receipts {
		if receipt.ReceiptID == plannerIDs.ReceiptID {
			plannerReceipt = receipt
		}
	}
	if string(plannerReceipt.Body) != string(body) || plannerReceipt.BodySHA256 != bytesHash(body) || !plannerReceipt.Normalized {
		t.Fatalf("raw receipt changed after normalization: hash=%q normalized=%v", plannerReceipt.BodySHA256, plannerReceipt.Normalized)
	}

	jsonExport, err := store.Export(ctx, QueryOptions{SnapshotID: snapshot.ID}, ExportJSON)
	if err != nil || !strings.Contains(string(jsonExport), `"month":"2020-01-01"`) {
		t.Fatalf("JSON export = %s, %v", jsonExport, err)
	}
	csvExport, err := store.Export(ctx, QueryOptions{SnapshotID: snapshot.ID}, ExportCSV)
	if err != nil || !strings.HasPrefix(string(csvExport), "keyword,geo,language_code,month,") {
		t.Fatalf("CSV export = %s, %v", csvExport, err)
	}
	if _, err := store.Export(ctx, QueryOptions{SnapshotID: snapshot.ID}, ExportFormat("yaml")); err == nil {
		t.Fatal("unsupported export format must fail")
	}

	listed, err := store.ListSnapshots(ctx, QueryOptions{Keyword: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) < 1 {
		t.Fatal("snapshot keyword search lost submitted/result evidence")
	}
	latest, err := store.ResolveSnapshot(ctx, "latest")
	if err != nil || latest != repeated.ID {
		t.Fatalf("latest = %q, %v; want second repeated snapshot %q", latest, err, repeated.ID)
	}
	previous, err := store.ResolveSnapshot(ctx, "previous")
	if err != nil || previous != snapshot.ID {
		t.Fatalf("previous = %q, %v; want first snapshot %q", previous, err, snapshot.ID)
	}
}

func TestNormalizationFailureKeepsRawAndCanFinalizeIncomplete(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       []byte
		wantMarker string
	}{
		{name: "malformed json", status: 200, body: []byte(`{"results":[`), wantMarker: "response_json_decode_failed"},
		{name: "no body error", status: 503, body: nil, wantMarker: "receipt_not_normalizable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			store := newTestStore(t)
			snapshot, err := store.StartSnapshot(ctx, testSnapshotInput(EndpointIdeas), time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			ids, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: snapshot.ID, Endpoint: EndpointIdeas, HTTPStatus: test.status, Body: test.body, SubmittedKeywords: []string{"seed"}, FetchedAt: time.Now().UTC()})
			if err != nil {
				t.Fatal(err)
			}
			result, err := store.NormalizeReceipt(ctx, ids.ReceiptID)
			if err == nil || !errors.Is(err, ErrInvalidResponse) || result.Complete {
				t.Fatalf("NormalizeReceipt = %#v, %v; want nonzero incomplete response error", result, err)
			}
			view, err := store.ShowSnapshot(ctx, snapshot.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(view.Receipts) != 1 || view.Receipts[0].Normalized || len(view.Coverage) != 1 || !strings.Contains(view.Coverage[0].CollectionStatus, test.wantMarker) {
				t.Fatalf("failure evidence = receipts=%#v coverage=%#v", view.Receipts, view.Coverage)
			}
			if err := store.Finish(ctx, FinishInput{SnapshotID: snapshot.ID, Complete: true}); err == nil || !errors.Is(err, ErrIncompleteSnapshot) {
				t.Fatalf("complete finish after permanent normalization failure = %v; want incomplete", err)
			}
			if err := store.Finish(ctx, FinishInput{SnapshotID: snapshot.ID, Status: StatusIncomplete, Complete: false, FailedReceipts: []string{ids.ReceiptID}}); err != nil {
				t.Fatalf("finalize incomplete: %v", err)
			}
			view, err = store.ShowSnapshot(ctx, snapshot.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !view.Snapshot.Finalized || view.Snapshot.Complete || view.Snapshot.Status != StatusIncomplete {
				t.Fatalf("incomplete finish = %#v", view.Snapshot)
			}
			if rows, err := store.Rows(ctx, QueryOptions{SnapshotID: snapshot.ID}); err != nil || len(rows) != 0 {
				t.Fatalf("incomplete analytical rows = %d, %v", len(rows), err)
			}
		})
	}
}

func TestValidEmptyResultIsDistinctFromFailureAndSearchableByInput(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	input := testSnapshotInput(EndpointIdeas)
	input.SubmittedSeeds = []string{"empty seed"}
	input.SubmittedKeywords = []string{"empty seed"}
	snapshot, err := store.StartSnapshot(ctx, input, time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"results":[],"unknownField":"kept"}`)
	ids, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: snapshot.ID, Endpoint: EndpointIdeas, SubmittedKeywords: []string{"empty seed"}, HTTPStatus: 200, Body: body, FetchedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.NormalizeReceipt(ctx, ids.ReceiptID)
	if err != nil || !result.Complete || result.KeywordRows != 0 || result.MonthlyRows != 0 {
		t.Fatalf("valid empty normalization = %#v, %v", result, err)
	}
	if err := store.Finish(ctx, FinishInput{SnapshotID: snapshot.ID, Complete: true}); err != nil {
		t.Fatal(err)
	}
	view, err := store.ShowSnapshot(ctx, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Rows) != 0 || len(view.Coverage) != 1 || !view.Coverage[0].NoResult || !view.Snapshot.Complete {
		t.Fatalf("valid empty view = rows=%d coverage=%#v snapshot=%#v", len(view.Rows), view.Coverage, view.Snapshot)
	}
	listed, err := store.ListSnapshots(ctx, QueryOptions{Keyword: "empty seed"})
	if err != nil || len(listed) != 1 {
		t.Fatalf("empty input inventory search = %d, %v; want one snapshot", len(listed), err)
	}
	if rows, err := store.Search(ctx, "empty seed", QueryOptions{}); err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("empty input analytical search = %#v, %v; want []", rows, err)
	}
}

func TestCloseVariantLineageDoesNotMultiplyRows(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	snapshot, err := store.StartSnapshot(ctx, testSnapshotInput(EndpointHistorical), time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"results":[{"text":"alpha","closeVariants":["alpha","beta","alpha variant"],"keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"11"}]}}]}`)
	ids, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: snapshot.ID, Endpoint: EndpointHistorical, SubmittedKeywords: []string{"alpha", "beta", "unmatched"}, HTTPStatus: 200, Body: body, FetchedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.NormalizeReceipt(ctx, ids.ReceiptID)
	if err != nil {
		t.Fatal(err)
	}
	if result.MonthlyRows != 1 || result.KeywordRows != 1 || result.VariantRows != 5 {
		t.Fatalf("close-variant normalization counts = %#v; want one metric/month and five lineage records", result)
	}
	if err := store.Finish(ctx, FinishInput{SnapshotID: snapshot.ID, Complete: true}); err != nil {
		t.Fatal(err)
	}
	rows, err := store.Rows(ctx, QueryOptions{SnapshotID: snapshot.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("close variants multiplied monthly rows: %d", len(rows))
	}
	if got, err := store.Search(ctx, "alpha variant", QueryOptions{SnapshotID: snapshot.ID}); err != nil || len(got) != 1 {
		t.Fatalf("close variant search = %d, %v; want one row", len(got), err)
	}
	if got, err := store.Search(ctx, "unmatched", QueryOptions{SnapshotID: snapshot.ID}); err != nil || len(got) != 0 {
		t.Fatalf("unmatched search = %d, %v; want no analytical row", len(got), err)
	}
	var closeCount, unmatchedCount, unknownIndexCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM variant_links WHERE metric_id = (SELECT id FROM keyword_metrics WHERE snapshot_id = ?) AND relationship = 'CLOSE_VARIANT'`, snapshot.ID).Scan(&closeCount); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM variant_links WHERE metric_id = (SELECT id FROM keyword_metrics WHERE snapshot_id = ?) AND relationship = 'UNMATCHED'`, snapshot.ID).Scan(&unmatchedCount); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM variant_links WHERE metric_id = (SELECT id FROM keyword_metrics WHERE snapshot_id = ?) AND relationship = 'UNKNOWN' AND submitted_index IS NULL`, snapshot.ID).Scan(&unknownIndexCount); err != nil {
		t.Fatal(err)
	}
	if closeCount != 3 || unmatchedCount != 1 || unknownIndexCount != 0 {
		t.Fatalf("variant link accounting = close=%d unmatched=%d unknown_without_index=%d", closeCount, unmatchedCount, unknownIndexCount)
	}
}

func TestRecoveredTransientAttemptCanFinishComplete(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	snapshot, err := store.StartSnapshot(ctx, testSnapshotInput(EndpointHistorical), time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: snapshot.ID, Endpoint: EndpointHistorical, PageNumber: 0, BatchNumber: 0, Attempt: 0, HTTPStatus: 429, ErrorCode: "RATE_EXCEEDED", ErrorMessage: "throttled", FetchedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"results":[{"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"1"}]}}]}`)
	ids, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: snapshot.ID, Endpoint: EndpointHistorical, PageNumber: 0, BatchNumber: 0, Attempt: 1, HTTPStatus: 200, Body: body, SubmittedKeywords: []string{"alpha"}, FetchedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NormalizeReceipt(ctx, ids.ReceiptID); err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, FinishInput{SnapshotID: snapshot.ID, Complete: true}); err != nil {
		t.Fatalf("recovered transient attempt could not finish complete: %v", err)
	}
	view, err := store.ShowSnapshot(ctx, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Snapshot.Complete || len(view.Receipts) != 2 || len(view.Coverage) != 1 {
		t.Fatalf("recovered snapshot = %#v receipts=%d coverage=%d", view.Snapshot, len(view.Receipts), len(view.Coverage))
	}
}

func TestNormalizationWriteFailureIsNotSuccessAndRawRemains(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	snapshot, err := store.StartSnapshot(ctx, testSnapshotInput(EndpointHistorical), time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"results":[{"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"1"}]}}]}`)
	ids, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: snapshot.ID, Endpoint: EndpointHistorical, HTTPStatus: 200, Body: body, SubmittedKeywords: []string{"alpha"}, FetchedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `CREATE TRIGGER reject_portfolio_metrics BEFORE INSERT ON keyword_metrics BEGIN SELECT RAISE(ABORT, 'fixture write rejection'); END`); err != nil {
		t.Fatal(err)
	}
	result, err := store.NormalizeReceipt(ctx, ids.ReceiptID)
	// The driver wraps the trigger as a SQLite constraint error; the key
	// contract is that the operation is nonzero and never reports success.
	if err == nil || result.Complete {
		t.Fatalf("write failure = %#v, %v; want non-success", result, err)
	}
	view, err := store.ShowSnapshot(ctx, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Receipts) != 1 || view.Receipts[0].Normalized || string(view.Receipts[0].Body) != string(body) || len(view.Rows) != 0 {
		t.Fatalf("write failure altered evidence = %#v", view)
	}
}

func TestCollectionSummaryUsesSQLCountsAndBoundedRows(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	snapshot, err := store.StartSnapshot(ctx, testSnapshotInput(EndpointHistorical), time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	accountIDs, err := store.AppendReceipt(ctx, ReceiptInput{
		SnapshotID: snapshot.ID,
		Endpoint:   EndpointAccount,
		HTTPStatus: 200,
		Body:       []byte(`{"results":[]}`),
		FetchedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAccountCurrency(ctx, snapshot.ID, "eur", "live_GAQL", accountIDs.ReceiptID); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"results":[
  {"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"1"}]}},
  {"text":"beta","keywordMetrics":{"monthlySearchVolumes":[{"month":"FEBRUARY","year":"2020","monthlySearches":"2"}]}},
  {"text":"gamma","keywordMetrics":{}}
]}`)
	ids, err := store.AppendReceipt(ctx, ReceiptInput{
		SnapshotID:        snapshot.ID,
		Endpoint:          EndpointHistorical,
		SubmittedKeywords: []string{"alpha", "beta"},
		HTTPStatus:        200,
		Body:              body,
		FetchedAt:         time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NormalizeReceipt(ctx, ids.ReceiptID); err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, FinishInput{SnapshotID: snapshot.ID, Complete: true}); err != nil {
		t.Fatal(err)
	}

	summary, err := store.CollectionSummary(ctx, "latest", 1)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Snapshot.ID != snapshot.ID || summary.Snapshot.CurrencyCode != "EUR" || summary.Snapshot.CurrencySource != "live_GAQL" || len(summary.Rows) != 1 || len(summary.Coverage) != 1 {
		t.Fatalf("bounded summary = snapshot=%q rows=%d coverage=%d", summary.Snapshot.ID, len(summary.Rows), len(summary.Coverage))
	}
	if summary.ReceiptCount != 2 || summary.StoredKeywordCount != 3 || summary.StoredMonthlyCount != 2 {
		t.Fatalf("summary counts = receipts=%d keywords=%d months=%d; want 2/3/2", summary.ReceiptCount, summary.StoredKeywordCount, summary.StoredMonthlyCount)
	}
	if summary.RenderedRowCount != 1 || summary.RenderLimit != 1 || !summary.RowsTruncated {
		t.Fatalf("summary rendering metadata = %#v", summary)
	}

	emptySnapshot, err := store.StartSnapshot(ctx, testSnapshotInput(EndpointIdeas), time.Date(2026, 9, 7, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	emptyIDs, err := store.AppendReceipt(ctx, ReceiptInput{
		SnapshotID:        emptySnapshot.ID,
		Endpoint:          EndpointIdeas,
		SubmittedKeywords: []string{"empty"},
		HTTPStatus:        200,
		Body:              []byte(`{"results":[]}`),
		FetchedAt:         time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NormalizeReceipt(ctx, emptyIDs.ReceiptID); err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, FinishInput{SnapshotID: emptySnapshot.ID, Complete: true}); err != nil {
		t.Fatal(err)
	}
	emptySummary, err := store.CollectionSummary(ctx, emptySnapshot.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if emptySummary.Rows == nil || emptySummary.Coverage == nil || len(emptySummary.Rows) != 0 || len(emptySummary.Coverage) != 1 || emptySummary.StoredKeywordCount != 0 || emptySummary.StoredMonthlyCount != 0 {
		t.Fatalf("empty summary = %#v", emptySummary)
	}
	if _, err := store.CollectionSummary(ctx, snapshot.ID, -1); err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative summary limit = %v; want invalid input", err)
	}
}

func TestOpenReadOnlyAndQueryValidation(t *testing.T) {
	ctx := context.Background()
	missing := filepath.Join(t.TempDir(), "missing.db")
	if _, err := OpenReadOnly(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("OpenReadOnly missing = %v; want ErrNotFound", err)
	}
	path := filepath.Join(t.TempDir(), "portfolio.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	version, err := store.SchemaVersion(ctx)
	if err != nil || version != portfolioSchemaVersion {
		t.Fatalf("schema version = %d, %v", version, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	readOnly, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	if _, err := readOnly.ListSnapshots(ctx, QueryOptions{Limit: -1}); err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative query limit = %v; want invalid input", err)
	}
	if _, err := readOnly.Rows(ctx, QueryOptions{StartMonth: "2020-03", EndMonth: "2020-01"}); err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("reversed query range = %v; want invalid input", err)
	}
	if rows, err := readOnly.Search(ctx, "", QueryOptions{}); err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("empty search = %#v, %v; want []", rows, err)
	}
}

func TestReadOnlyHandleSeesCommittedWriterReceipts(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "portfolio.db")
	writer, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	reader, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	snapshot, err := writer.StartSnapshot(ctx, testSnapshotInput(EndpointHistorical), time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	listed, err := reader.ListSnapshots(ctx, QueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != snapshot.ID {
		t.Fatalf("read-only reader did not observe committed snapshot: %#v", listed)
	}
	body := []byte(`{"results":[]}`)
	if _, err := writer.AppendReceipt(ctx, ReceiptInput{SnapshotID: snapshot.ID, Endpoint: EndpointHistorical, HTTPStatus: 200, Body: body, SubmittedKeywords: []string{"seed"}, FetchedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	view, err := reader.ShowSnapshot(ctx, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Receipts) != 1 || string(view.Receipts[0].Body) != string(body) {
		t.Fatalf("read-only reader did not observe committed receipt: %#v", view.Receipts)
	}
	if _, err := reader.StartSnapshot(ctx, testSnapshotInput(EndpointIdeas), time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)); err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("read-only write attempt = %v; want read-only invalid input", err)
	}
}

func TestStartAndAppendValidationTable(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	fetchedAt := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name  string
		input SnapshotInput
	}{
		{name: "unsupported endpoint", input: SnapshotInput{Endpoint: "campaigns", RequestedStart: "2020-01", RequestedEnd: "2020-02"}},
		{name: "open end", input: SnapshotInput{Endpoint: EndpointIdeas, RequestedStart: "2020-01", RequestedEnd: "2026-09"}},
		{name: "bad network", input: SnapshotInput{Endpoint: EndpointIdeas, Network: "SEARCH_PARTNERS", RequestedStart: "2020-01", RequestedEnd: "2020-02"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.StartSnapshot(ctx, test.input, fetchedAt); err == nil || !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("StartSnapshot error = %v; want invalid input", err)
			}
		})
	}
	snapshot, err := store.StartSnapshot(ctx, testSnapshotInput(EndpointIdeas), fetchedAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: snapshot.ID, Endpoint: EndpointIdeas, PageNumber: -1}); err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative receipt page = %v; want invalid input", err)
	}
	if err := store.Finish(ctx, FinishInput{SnapshotID: snapshot.ID, Complete: true}); err == nil || !errors.Is(err, ErrIncompleteSnapshot) {
		t.Fatalf("finish without receipt = %v; want incomplete snapshot", err)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "portfolio.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func testSnapshotInput(endpoint string) SnapshotInput {
	return SnapshotInput{
		RunID:              "test-run",
		Endpoint:           endpoint,
		APIVersion:         "v25",
		DiscoveryRevision:  "test-discovery",
		CustomerID:         "1234567890",
		LoginCustomerID:    "1234567890",
		Language:           "languageConstants/1000",
		Network:            NetworkGoogleSearch,
		GeoTargetConstants: []string{"geoTargetConstants/2840"},
		SubmittedSeeds:     []string{"seed"},
		SubmittedKeywords:  []string{"seed"},
		RequestedStart:     "2020-01",
		RequestedEnd:       "2020-02",
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

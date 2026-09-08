// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestTraceReportLinksValueToReceiptAndMarksAmbiguity(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	snapshot, err := store.StartSnapshot(ctx, testSnapshotInput(EndpointHistorical), time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"results":[{"text":"alpha","keywordMetrics":{"avgMonthlySearches":"0","monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"0"}]}}]}`)
	requestBody := json.RawMessage(`{"keywords":["alpha"]}`)
	first, err := store.AppendReceipt(ctx, ReceiptInput{
		SnapshotID:        snapshot.ID,
		Endpoint:          EndpointHistorical,
		PageNumber:        0,
		RequestBody:       requestBody,
		SubmittedKeywords: []string{"alpha"},
		HTTPStatus:        200,
		GoogleRequestID:   "trace-request-1",
		Body:              body,
		FetchedAt:         time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NormalizeReceipt(ctx, first.ReceiptID); err != nil {
		t.Fatal(err)
	}

	trace, err := store.Trace(ctx, snapshot.ID, TraceOptions{Keyword: "ALPHA", Month: "2020-01"})
	if err != nil {
		t.Fatal(err)
	}
	if trace.Resolution != "unique" || trace.MatchCount != 1 || trace.Ambiguous || len(trace.Matches) != 1 {
		t.Fatalf("unique trace = %#v", trace)
	}
	match := trace.Matches[0]
	if match.Row.Month != "2020-01-01" || match.Row.MonthlySearches == nil || *match.Row.MonthlySearches != 0 {
		t.Fatalf("traced value = %#v", match.Row)
	}
	if match.Receipt.ReceiptID != first.ReceiptID || match.Receipt.BodySHA256 != bytesHash(body) || !match.Receipt.BodyPresent {
		t.Fatalf("receipt identity = %#v", match.Receipt)
	}
	if !match.Receipt.RequestBodyPresent || string(match.Receipt.RequestBody) != string(requestBody) || len(match.Receipt.RawBody) != 0 {
		t.Fatalf("bounded receipt body = %#v", match.Receipt)
	}
	if !match.Metric.RawResultPresent || match.Metric.MetricID != match.Row.MetricID || match.Metric.ReceiptID != first.ReceiptID {
		t.Fatalf("metric linkage = %#v", match.Metric)
	}

	withRaw, err := store.Trace(ctx, snapshot.ID, TraceOptions{Keyword: "alpha", Month: "2020-01", IncludeRaw: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(withRaw.Matches) != 1 || string(withRaw.Matches[0].Receipt.RawBody) != string(body) {
		t.Fatalf("raw trace = %#v", withRaw)
	}

	secondBody := []byte(`{"results":[{"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"3"}]}}]}`)
	second, err := store.AppendReceipt(ctx, ReceiptInput{
		SnapshotID:        snapshot.ID,
		Endpoint:          EndpointHistorical,
		PageNumber:        1,
		SubmittedKeywords: []string{"alpha"},
		HTTPStatus:        200,
		Body:              secondBody,
		FetchedAt:         time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NormalizeReceipt(ctx, second.ReceiptID); err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, FinishInput{SnapshotID: snapshot.ID, Complete: true}); err != nil {
		t.Fatal(err)
	}
	ambiguous, err := store.Trace(ctx, "latest", TraceOptions{Keyword: "alpha", Month: "2020-01"})
	if err != nil {
		t.Fatal(err)
	}
	if ambiguous.Resolution != "ambiguous" || ambiguous.MatchCount != 2 || !ambiguous.Ambiguous || len(ambiguous.Matches) != 2 || !hasString(ambiguous.Warnings, "multiple_normalized_monthly_matches") {
		t.Fatalf("ambiguous trace = %#v", ambiguous)
	}

	for _, invalid := range []TraceOptions{{Month: "2020-01"}, {Keyword: "alpha", Month: "2020-13"}} {
		if _, err := store.Trace(ctx, snapshot.ID, invalid); err == nil || !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid trace options %#v = %v; want invalid input", invalid, err)
		}
	}
	noMatch, err := store.Trace(ctx, snapshot.ID, TraceOptions{Keyword: "missing", Month: "2020-01"})
	if err != nil {
		t.Fatal(err)
	}
	if noMatch.Matches == nil || noMatch.Resolution != "none" || !hasString(noMatch.Warnings, "no_normalized_monthly_match") {
		t.Fatalf("empty trace = %#v", noMatch)
	}
}

func TestCoverageReportSeparatesMetricAndPageCoverage(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	input := testSnapshotInput(EndpointHistorical)
	input.SubmittedKeywords = []string{"alpha", "gamma", "delta"}
	input.RequestedStart = "2020-01"
	input.RequestedEnd = "2020-02"
	snapshot, err := store.StartSnapshot(ctx, input, time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"results":[
  {"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":null},{"month":"FEBRUARY","year":"2020","monthlySearches":"0"}]}},
  {"text":"gamma","keywordMetrics":{}},
  {"text":"delta","keywordMetrics":{"monthlySearchVolumes":[{"month":"SEPTEMBER","year":"2099","monthlySearches":"4"}]}}
]}`)
	ids, err := store.AppendReceipt(ctx, ReceiptInput{
		SnapshotID:        snapshot.ID,
		Endpoint:          EndpointHistorical,
		PageNumber:        0,
		SubmittedKeywords: input.SubmittedKeywords,
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

	report, err := store.CoverageReport(ctx, "latest", CoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Keywords) != 3 || len(report.Pages) != 1 || report.RequestedCount != 3 || report.ReturnedCount != 3 || report.NoResult || report.NoResultBatches != 0 || report.IncompletePages != 0 || !report.Complete {
		t.Fatalf("coverage summary = %#v", report)
	}
	if !report.ShortWindow || len(report.RequestedMonths) != 2 || len(report.ReturnedMonths) != 2 || !hasString(report.RawOnlyMonths, "2099-09-01") {
		t.Fatalf("coverage month summary = %#v", report)
	}
	byText := make(map[string]KeywordCoverage, len(report.Keywords))
	for _, item := range report.Keywords {
		byText[item.ReturnedText] = item
	}
	alpha := byText["alpha"]
	if !reflectStringSet(alpha.ReturnedMonths, []string{"2020-01-01", "2020-02-01"}) || !reflectStringSet(alpha.UnavailableMonths, []string{"2020-01-01"}) || len(alpha.MissingMonths) != 0 || alpha.ShortWindow {
		t.Fatalf("alpha metric coverage = %#v", alpha)
	}
	gamma := byText["gamma"]
	if len(gamma.ReturnedMonths) != 0 || len(gamma.MissingMonths) != 2 || !gamma.ShortWindow || len(gamma.RequestedMonths) != 2 {
		t.Fatalf("no-series metric coverage = %#v", gamma)
	}
	delta := byText["delta"]
	if len(delta.ReturnedMonths) != 0 || !reflectStringSet(delta.RawOnlyMonths, []string{"2099-09-01"}) || len(delta.MissingMonths) != 2 || !delta.ShortWindow {
		t.Fatalf("raw-only metric coverage = %#v", delta)
	}
	if !reflectStringSet(report.Pages[0].RawOnlyMonths, []string{"2099-09-01"}) || !reflectStringSet(report.Pages[0].ReturnedMonths, []string{"2020-01-01", "2020-02-01"}) {
		t.Fatalf("page coverage = %#v", report.Pages[0])
	}

	emptyInput := testSnapshotInput(EndpointIdeas)
	emptyInput.SubmittedSeeds = []string{"empty"}
	emptyInput.SubmittedKeywords = nil
	emptySnapshot, err := store.StartSnapshot(ctx, emptyInput, time.Date(2026, 9, 7, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	emptyIDs, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: emptySnapshot.ID, Endpoint: EndpointIdeas, SubmittedKeywords: []string{"empty"}, HTTPStatus: 200, Body: []byte(`{"results":[]}`), FetchedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NormalizeReceipt(ctx, emptyIDs.ReceiptID); err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, FinishInput{SnapshotID: emptySnapshot.ID, Complete: true}); err != nil {
		t.Fatal(err)
	}
	emptyReport, err := store.CoverageReport(ctx, emptySnapshot.ID, CoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if emptyReport.Pages == nil || len(emptyReport.Pages) != 1 || !emptyReport.Pages[0].NoResult || emptyReport.NoResultBatches != 1 || !emptyReport.NoResult || !emptyReport.Complete {
		t.Fatalf("empty coverage = %#v", emptyReport)
	}

	failedInput := testSnapshotInput(EndpointIdeas)
	failedInput.SubmittedSeeds = []string{"failed"}
	failedSnapshot, err := store.StartSnapshot(ctx, failedInput, time.Date(2026, 9, 7, 12, 2, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	failedIDs, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: failedSnapshot.ID, Endpoint: EndpointIdeas, SubmittedKeywords: []string{"failed"}, HTTPStatus: 200, Body: []byte(`{"results":[`), FetchedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NormalizeReceipt(ctx, failedIDs.ReceiptID); err == nil || !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("malformed coverage normalization = %v", err)
	}
	if err := store.Finish(ctx, FinishInput{SnapshotID: failedSnapshot.ID, Status: StatusIncomplete, Complete: false, FailedReceipts: []string{failedIDs.ReceiptID}}); err != nil {
		t.Fatal(err)
	}
	failedReport, err := store.CoverageReport(ctx, failedSnapshot.ID, CoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if failedReport.Complete || failedReport.IncompletePages != 1 || len(failedReport.Pages) != 1 || failedReport.Pages[0].Complete || !hasString(failedReport.Warnings, "incomplete_collection") {
		t.Fatalf("failed coverage = %#v", failedReport)
	}
}

func TestVariantsReportPreservesOrderAndDoesNotMultiplyRows(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	input := testSnapshotInput(EndpointHistorical)
	input.SubmittedKeywords = []string{"alpha", "beta", "unmatched"}
	snapshot, err := store.StartSnapshot(ctx, input, time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"results":[
  {"text":"alpha","closeVariants":["alpha","alpha variant"],"keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"11"}]}},
  {"text":"beta","keywordMetrics":{"monthlySearchVolumes":[{"month":"FEBRUARY","year":"2020","monthlySearches":"12"}]}}
]}`)
	ids, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: snapshot.ID, Endpoint: EndpointHistorical, SubmittedKeywords: input.SubmittedKeywords, HTTPStatus: 200, Body: body, FetchedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NormalizeReceipt(ctx, ids.ReceiptID); err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, FinishInput{SnapshotID: snapshot.ID, Complete: true}); err != nil {
		t.Fatal(err)
	}

	report, err := store.VariantsReport(ctx, "latest", VariantsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflectSubmitted(report.Submitted, []SubmittedTerm{{Index: 0, Text: "alpha", Kind: "keyword"}, {Index: 1, Text: "beta", Kind: "keyword"}, {Index: 2, Text: "unmatched", Kind: "keyword"}}) {
		t.Fatalf("submitted order = %#v", report.Submitted)
	}
	if report.MetricCount != 2 || report.MonthlyRowCount != 2 || report.LinkCount != 8 || report.ExactCount != 2 || report.CloseVariantCount != 2 || report.UnmatchedCount != 4 || report.UnknownCount != 0 || len(report.Metrics) != 2 {
		t.Fatalf("variant counts = %#v", report)
	}
	for _, metric := range report.Metrics {
		if len(metric.Rows) != 1 {
			t.Fatalf("metric %s rows=%d; close variants must not multiply rows", metric.ReturnedText, len(metric.Rows))
		}
	}
	var closeWithoutIndex, unmatchedBeta bool
	for _, link := range report.Links {
		if link.Relationship == "CLOSE_VARIANT" && link.SubmittedText == "alpha variant" {
			closeWithoutIndex = link.SubmittedIndex == nil
		}
		if link.Relationship == "UNMATCHED" && link.SubmittedText == "beta" {
			unmatchedBeta = link.SubmittedIndex != nil && *link.SubmittedIndex == 1
		}
	}
	if !closeWithoutIndex || !unmatchedBeta {
		t.Fatalf("variant index handling = close_without_index=%v unmatched_beta=%v links=%#v", closeWithoutIndex, unmatchedBeta, report.Links)
	}
	filtered, err := store.VariantsReport(ctx, snapshot.ID, VariantsOptions{Keyword: "unmatched"})
	if err != nil || filtered.MetricCount != 2 || filtered.LinkCount != 8 {
		t.Fatalf("filtered variants = metrics=%d links=%d err=%v", filtered.MetricCount, filtered.LinkCount, err)
	}

	ideasInput := testSnapshotInput(EndpointIdeas)
	ideasInput.SubmittedSeeds = []string{"alpha", "beta"}
	ideasInput.SubmittedKeywords = nil
	ideas, err := store.StartSnapshot(ctx, ideasInput, time.Date(2026, 9, 7, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	ideaBody := []byte(`{"results":[
  {"text":"combined result","closeVariants":["alpha","beta"],"keywordIdeaMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"9"}]}},
  {"text":"unrelated result","keywordIdeaMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"10"}]}}
]}`)
	ideaIDs, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: ideas.ID, Endpoint: EndpointIdeas, SubmittedKeywords: ideasInput.SubmittedSeeds, HTTPStatus: 200, Body: ideaBody, FetchedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NormalizeReceipt(ctx, ideaIDs.ReceiptID); err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, FinishInput{SnapshotID: ideas.ID, Complete: true}); err != nil {
		t.Fatal(err)
	}
	ideaReport, err := store.VariantsReport(ctx, ideas.ID, VariantsOptions{Keyword: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if !ideaReport.CombinedRequest || ideaReport.AttributionStatus != "combined_request_no_per_seed_attribution" || ideaReport.MetricCount != 1 || ideaReport.MonthlyRowCount != 1 || len(ideaReport.Submitted) != 2 || len(ideaReport.Links) != 2 || ideaReport.Metrics[0].ReturnedText != "combined result" {
		t.Fatalf("combined Ideas report = %#v", ideaReport)
	}
	for _, link := range ideaReport.Links {
		if link.SubmittedIndex != nil {
			t.Fatalf("Ideas link invented per-seed attribution: %#v", link)
		}
	}
	allIdeas, err := store.VariantsReport(ctx, ideas.ID, VariantsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if allIdeas.MetricCount != 2 || allIdeas.MonthlyRowCount != 2 {
		t.Fatalf("unfiltered Ideas report = %#v", allIdeas)
	}
}

func reflectStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, value := range want {
		if !hasString(got, value) {
			return false
		}
	}
	return true
}

func reflectSubmitted(got, want []SubmittedTerm) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

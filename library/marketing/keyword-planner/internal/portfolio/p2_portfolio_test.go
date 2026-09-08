// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestP2DiffReportsRangeAndObservationChangesWithoutDeduplication(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	leftInput := testSnapshotInput(EndpointHistorical)
	leftInput.RequestedStart = "2020-01"
	leftInput.RequestedEnd = "2020-02"
	leftInput.RequestBody = json.RawMessage(`{"keywords":["alpha"],"end":"2020-02"}`)
	left := p2NormalizedSnapshot(t, store, leftInput, []byte(`{"results":[{"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"10"},{"month":"FEBRUARY","year":"2020","monthlySearches":"20"}]}}]}`), true)

	rightInput := leftInput
	rightInput.RequestedEnd = "2020-03"
	rightInput.RequestBody = json.RawMessage(`{"keywords":["alpha"],"end":"2020-03"}`)
	right := p2NormalizedSnapshot(t, store, rightInput, []byte(`{"results":[{"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"12"},{"month":"FEBRUARY","year":"2020","monthlySearches":"20"},{"month":"MARCH","year":"2020","monthlySearches":"30"}]}}]}`), true)

	diff, err := store.Diff(ctx, left.ID, right.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Comparable || !diff.RangeChanged || len(diff.Changes) != 2 {
		t.Fatalf("range diff = %#v", diff)
	}
	if !p2DiffRequestChanged(diff.RequestChanges, "requested_end") {
		t.Fatalf("request changes = %#v", diff.RequestChanges)
	}
	var changed, added DiffChange
	for _, item := range diff.Changes {
		switch item.Kind {
		case "changed":
			changed = item
		case "added":
			added = item
		}
	}
	if changed.Kind != "changed" || changed.Key.Month != "2020-01-01" || !p2HasString(changed.ChangedFields, "monthly_searches") {
		t.Fatalf("changed January row = %#v", changed)
	}
	if added.Kind != "added" || added.Key.Month != "2020-03-01" || added.Reason != "newly_included_month" || !strings.Contains(added.ScopeNote, "not evidence") {
		t.Fatalf("added range row = %#v", added)
	}
	if diff.Summary.UnchangedRows != 1 || diff.Summary.ChangedRows != 1 || diff.Summary.AddedRows != 1 {
		t.Fatalf("diff summary = %#v", diff.Summary)
	}

	same, err := store.Diff(ctx, right.ID, right.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !same.Comparable || len(same.Changes) != 0 || len(same.CoverageChanges) != 0 || same.Summary.UnchangedRows != 3 {
		t.Fatalf("same snapshot diff = %#v", same)
	}

	mismatchInput := rightInput
	mismatchInput.Language = "languageConstants/1001"
	mismatch := p2NormalizedSnapshot(t, store, mismatchInput, []byte(`{"results":[{"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"12"}]}}]}`), true)
	incomparable, err := store.Diff(ctx, left.ID, mismatch.ID)
	if err == nil || !errors.Is(err, ErrIncomparableSnapshots) || incomparable.Comparable || len(incomparable.Incompatibilities) != 1 || incomparable.Incompatibilities[0] != "language" {
		t.Fatalf("incomparable diff = %#v, %v", incomparable, err)
	}
}

func TestP2DiffRetainsNullableMonthlyStateAndFlags(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	input := testSnapshotInput(EndpointHistorical)
	input.RequestedStart = "2020-01"
	input.RequestedEnd = "2020-01"
	nullSnapshot := p2NormalizedSnapshot(t, store, input, []byte(`{"results":[{"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":null}]}}]}`), true)
	valueSnapshot := p2NormalizedSnapshot(t, store, input, []byte(`{"results":[{"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"7"}]}}]}`), true)

	diff, err := store.Diff(ctx, nullSnapshot.ID, valueSnapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Changes) != 1 || diff.Changes[0].Kind != "changed" || diff.Changes[0].Left.MonthlySearches != nil || diff.Changes[0].Right.MonthlySearches == nil || *diff.Changes[0].Right.MonthlySearches != 7 || !p2HasString(diff.Changes[0].ChangedFields, "monthly_searches") || !p2HasString(diff.Changes[0].ChangedFields, "value_state") {
		t.Fatalf("nullable monthly diff = %#v", diff)
	}
}

func TestP2DiffLabelsMonthsRemovedByNarrowerRightRangeAsScope(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	wide := testSnapshotInput(EndpointHistorical)
	wide.RequestedStart = "2020-01"
	wide.RequestedEnd = "2020-02"
	narrow := wide
	narrow.RequestedEnd = "2020-01"
	left := p2NormalizedSnapshot(t, store, wide, []byte(`{"results":[{"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"10"},{"month":"FEBRUARY","year":"2020","monthlySearches":"20"}]}}]}`), true)
	right := p2NormalizedSnapshot(t, store, narrow, []byte(`{"results":[{"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"10"}]}}]}`), true)

	diff, err := store.Diff(ctx, left.ID, right.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Changes) != 1 || diff.Changes[0].Kind != "removed" || diff.Changes[0].Key.Month != "2020-02-01" || diff.Changes[0].Reason != "outside_right_requested_range" || !strings.Contains(diff.Changes[0].ScopeNote, "not evidence of lost demand") {
		t.Fatalf("narrower-range diff = %#v", diff)
	}
}

func TestP2IntegrityDetectsRawAndNormalizedTampering(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	input := testSnapshotInput(EndpointHistorical)
	input.RequestedStart = "2020-01"
	input.RequestedEnd = "2020-02"
	body := []byte(`{"results":[{"text":"alpha","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"10"},{"month":"FEBRUARY","year":"2020","monthlySearches":null}]}}]}`)
	snapshot := p2NormalizedSnapshot(t, store, input, body, true)
	valid, err := store.Integrity(ctx, snapshot.ID)
	if err != nil || !valid.Valid || len(valid.Issues) != 0 {
		t.Fatalf("valid integrity = %#v, %v", valid, err)
	}

	var receiptID string
	if err := store.DB().QueryRowContext(ctx, `SELECT id FROM response_receipts WHERE snapshot_id = ? AND endpoint = ?`, snapshot.ID, EndpointHistorical).Scan(&receiptID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE response_receipts SET body = ? WHERE id = ?`, []byte(`{"results":[]}`), receiptID); err != nil {
		t.Fatal(err)
	}
	badRaw, err := store.Integrity(ctx, snapshot.ID)
	if err == nil || !errors.Is(err, ErrIntegrity) || badRaw.Valid || !p2IntegrityHasCheck(badRaw.Issues, "receipt_hashes") {
		t.Fatalf("raw tamper integrity = %#v, %v", badRaw, err)
	}

	store2 := newTestStore(t)
	snapshot2 := p2NormalizedSnapshot(t, store2, input, body, true)
	var monthlyID int64
	if err := store2.DB().QueryRowContext(ctx, `SELECT id FROM monthly_volumes WHERE snapshot_id = ? ORDER BY id LIMIT 1`, snapshot2.ID).Scan(&monthlyID); err != nil {
		t.Fatal(err)
	}
	if _, err := store2.DB().ExecContext(ctx, `UPDATE monthly_volumes SET monthly_searches = ? WHERE id = ?`, int64(99), monthlyID); err != nil {
		t.Fatal(err)
	}
	badProjection, err := store2.Integrity(ctx, snapshot2.ID)
	if err == nil || !errors.Is(err, ErrIntegrity) || badProjection.Valid || !p2IntegrityHasCheck(badProjection.Issues, "raw_projection") {
		t.Fatalf("normalized tamper integrity = %#v, %v", badProjection, err)
	}
}

func TestP2SafeStatsExcludesUnsafeValuesAndKeepsObservationScope(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	input := testSnapshotInput(EndpointHistorical)
	input.RequestedStart = "2020-01"
	input.RequestedEnd = "2020-04"
	body := []byte(`{"results":[
		{"text":"alpha","closeVariants":["alpha close"],"keywordMetrics":{"avgMonthlySearches":"100","competition":"HIGH","monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"10"},{"month":"FEBRUARY","year":"2020","monthlySearches":null},{"month":"MARCH","year":"2020","monthlySearches":"-3"}]}},
		{"text":"ambiguous","keywordMetrics":{"avgMonthlySearches":"0","competition":"UNSPECIFIED","monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"7"}]}},
		{"text":"negative","keywordMetrics":{"avgMonthlySearches":"20","competition":"HIGH","monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"-4"}]}}
	]}`)
	snapshot := p2NormalizedSnapshot(t, store, input, body, true)
	stats, err := store.SafeStats(ctx, SafeStatsOptions{SnapshotID: snapshot.ID})
	if err != nil {
		t.Fatal(err)
	}
	if stats.RowsExamined != 5 || stats.RowsIncluded != 1 || stats.ExcludedRows != 4 || stats.ExcludedNull != 1 || stats.ExcludedNegative != 2 || stats.ExcludedAmbiguousZero != 1 || stats.NoEligibleValues {
		t.Fatalf("safe stats accounting = %#v", stats)
	}
	var alpha, ambiguous SafeStatsGroup
	for _, group := range stats.Groups {
		switch group.Keyword {
		case "alpha":
			alpha = group
		case "ambiguous":
			ambiguous = group
		}
	}
	if alpha.Keyword != "alpha" || alpha.SumMonthlySearches != "10" || alpha.ObservedMean != "10" || alpha.ObservedMonths != 1 || alpha.NullRows != 1 || alpha.ExcludedRows != 2 || !alpha.ShortWindow || !p2HasString(alpha.Flags, "PLANNER-SHORT-WINDOW") {
		t.Fatalf("alpha stats = %#v", alpha)
	}
	if ambiguous.Keyword != "ambiguous" || !ambiguous.NoEligibleValues || ambiguous.SumMonthlySearches != "" || ambiguous.ExcludedRows != 1 {
		t.Fatalf("ambiguous stats = %#v", ambiguous)
	}
	if len(stats.Groups) != 3 {
		t.Fatalf("variant or metric rows were collapsed unexpectedly: %#v", stats.Groups)
	}

	overflowInput := input
	overflowInput.RequestedEnd = "2020-02"
	overflowBody := []byte(`{"results":[{"text":"large","keywordMetrics":{"monthlySearchVolumes":[{"month":"JANUARY","year":"2020","monthlySearches":"9223372036854775807"},{"month":"FEBRUARY","year":"2020","monthlySearches":"1"}]}}]}`)
	overflowSnapshot := p2NormalizedSnapshot(t, store, overflowInput, overflowBody, true)
	overflow, err := store.SafeStats(ctx, SafeStatsOptions{SnapshotID: overflowSnapshot.ID})
	if err != nil || len(overflow.Groups) != 1 || overflow.Groups[0].SumMonthlySearches != "9223372036854775808" {
		t.Fatalf("overflow stats = %#v, %v", overflow, err)
	}

	incomplete := p2NormalizedSnapshot(t, store, input, body, false)
	incompleteStats, err := store.SafeStats(ctx, SafeStatsOptions{SnapshotID: incomplete.ID})
	if err != nil || incompleteStats.RowsIncluded != 0 || incompleteStats.ExcludedIncomplete != 5 || !incompleteStats.NoEligibleValues {
		t.Fatalf("incomplete stats = %#v, %v", incompleteStats, err)
	}
}

func p2NormalizedSnapshot(t *testing.T, store *Store, input SnapshotInput, body []byte, finish bool) Snapshot {
	t.Helper()
	ctx := context.Background()
	snapshot, err := store.StartSnapshot(ctx, input, time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	ids, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: snapshot.ID, Endpoint: input.Endpoint, PageNumber: 0, BatchNumber: 0, Attempt: 0, RequestBody: input.RequestBody, SubmittedKeywords: input.SubmittedKeywords, HTTPStatus: 200, Body: body, FetchedAt: time.Date(2026, 9, 7, 12, 0, 1, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NormalizeReceipt(ctx, ids.ReceiptID); err != nil {
		t.Fatal(err)
	}
	if finish {
		if err := store.Finish(ctx, FinishInput{SnapshotID: snapshot.ID, Complete: true}); err != nil {
			t.Fatal(err)
		}
	}
	return snapshot
}

func p2DiffRequestChanged(changes []DiffFieldChange, field string) bool {
	for _, change := range changes {
		if change.Field == field {
			return true
		}
	}
	return false
}

func p2IntegrityHasCheck(issues []IntegrityIssue, check string) bool {
	for _, issue := range issues {
		if issue.Check == check {
			return true
		}
	}
	return false
}

func p2HasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

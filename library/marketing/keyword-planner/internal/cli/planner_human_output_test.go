// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/portfolio"
)

func TestPlannerHumanCollectionOutputShowsCountsTruncationAndUnavailable(t *testing.T) {
	large := int64(9223372036854775807)
	value := plannerCollectionOutput{
		Snapshot: portfolio.Snapshot{
			ID:             "snapshot-1",
			Endpoint:       portfolio.EndpointHistorical,
			Status:         portfolio.StatusIncomplete,
			Complete:       false,
			CurrencyCode:   "EUR",
			RequestedStart: "2025-01",
			RequestedEnd:   "2025-02",
		},
		Rows: []portfolio.Row{
			{Keyword: "exact", Month: "2025-01-01", MonthlySearches: &large, Complete: false, Status: "ok"},
			{Keyword: "missing", Month: "2025-02-01", MonthlySearches: nil, Flags: []string{"ambiguous_zero_unspecified"}, Complete: false, Status: "ok"},
		},
		ReceiptCount:       2,
		StoredKeywordCount: 2,
		StoredMonthlyCount: 4,
		RenderedRowCount:   2,
		RenderLimit:        2,
		RowsTruncated:      true,
		Warnings:           []string{"incomplete_collection"},
	}

	var out bytes.Buffer
	if err := writePlannerCollectionHuman(&out, value); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"Status: incomplete; complete=incomplete",
		"Counts: receipts=2; keywords=2; monthly_rows=4",
		"Rows: showing 2 of 4 (limit=2; truncated)",
		"KEYWORD",
		"MONTHLY_SEARCHES",
		"9223372036854775807",
		"unavailable",
		"ambiguous_zero_unspecified",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("human collection output missing %q:\n%s", want, text)
		}
	}
}

func TestPlannerHumanPortfolioTablesAreStableAndEmptyIsReadable(t *testing.T) {
	snapshots := []portfolio.Snapshot{{
		ID:             "snapshot-1",
		Endpoint:       portfolio.EndpointIdeas,
		Status:         portfolio.StatusComplete,
		Complete:       true,
		SubmittedSeeds: []string{"seed one", "seed two"},
		RequestedStart: "2025-01",
		RequestedEnd:   "2025-12",
		FetchedAt:      time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC),
	}}
	var snapshotOut bytes.Buffer
	if err := writePlannerSnapshotsHuman(&snapshotOut, snapshots); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"SNAPSHOT", "ENDPOINT", "COMPLETE", "snapshot-1", "complete", "2", "2025-01..2025-12"} {
		if !strings.Contains(snapshotOut.String(), want) {
			t.Fatalf("snapshot table missing %q:\n%s", want, snapshotOut.String())
		}
	}

	var emptyOut bytes.Buffer
	if err := writePlannerRowsHuman(&emptyOut, []portfolio.Row{}, "Monthly rows"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(emptyOut.String(), "(no monthly rows)") {
		t.Fatalf("empty rows output = %q", emptyOut.String())
	}
}

func TestPlannerHumanTerminalRequiresTerminalAndNoMachineFlags(t *testing.T) {
	var out bytes.Buffer
	if plannerWantsHumanTerminal(&out, &rootFlags{}) {
		t.Fatal("piped default output selected human terminal renderer")
	}
	for _, flags := range []*rootFlags{
		{asJSON: true}, {agent: true}, {csv: true}, {plain: true}, {compact: true}, {quiet: true}, {selectFields: "keyword"},
	} {
		if plannerWantsHumanTerminal(&out, flags) {
			t.Fatalf("machine flags selected human renderer: %#v", flags)
		}
	}
}

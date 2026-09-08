// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/portfolio"
)

type plannerViewFixtureData struct {
	dbPath string
	first  portfolio.Snapshot
	latest portfolio.Snapshot
}

func newPlannerViewFixture(t *testing.T) plannerViewFixtureData {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "snapshots.db")
	store, err := portfolio.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}

	makeSnapshot := func(runID string, fetchedAt time.Time, steakValue int64) portfolio.Snapshot {
		input := portfolio.SnapshotInput{
			RunID:              runID,
			Endpoint:           portfolio.EndpointHistorical,
			APIVersion:         "v25",
			DiscoveryRevision:  "fixture-discovery",
			CustomerID:         "fixture-customer",
			LoginCustomerID:    "fixture-login",
			Language:           "languageConstants/1000",
			Network:            portfolio.NetworkGoogleSearch,
			CurrencyCode:       "EUR",
			CurrencySource:     "fixture",
			GeoTargetConstants: []string{"geoTargetConstants/2840"},
			SubmittedSeeds:     []string{"beef steak", "beef brisket"},
			SubmittedKeywords:  []string{"beef steak", "beef brisket"},
			RequestedStart:     "2026-01",
			RequestedEnd:       "2026-07",
			RequestBody:        json.RawMessage(`{"keywords":["beef steak","beef brisket"]}`),
			SourceVariant:      "fixture",
		}
		snapshot, err := store.StartSnapshot(ctx, input, fetchedAt)
		if err != nil {
			t.Fatal(err)
		}
		body := []byte(fmt.Sprintf(`{"results":[{"text":"beef steak","closeVariants":["beef steak","steak beef"],"keywordMetrics":{"avgMonthlySearches":"%d","monthlySearchVolumes":[{"month":"JULY","year":"2026","monthlySearches":"%d"}]}},{"text":"beef brisket","keywordMetrics":{"monthlySearchVolumes":[{"month":"JULY","year":"2026","monthlySearches":"20"}]}}]}`, steakValue, steakValue))
		ids, err := store.AppendReceipt(ctx, portfolio.ReceiptInput{
			SnapshotID:        snapshot.ID,
			Endpoint:          portfolio.EndpointHistorical,
			PageNumber:        0,
			BatchNumber:       0,
			Attempt:           0,
			RequestBody:       input.RequestBody,
			SubmittedKeywords: input.SubmittedKeywords,
			HTTPStatus:        200,
			GoogleRequestID:   "fixture-request-" + runID,
			Body:              body,
			FetchedAt:         fetchedAt,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.NormalizeReceipt(ctx, ids.ReceiptID); err != nil {
			t.Fatal(err)
		}
		if err := store.Finish(ctx, portfolio.FinishInput{SnapshotID: snapshot.ID, Complete: true}); err != nil {
			t.Fatal(err)
		}
		return snapshot
	}

	first := makeSnapshot("fixture-first", time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC), 10)
	latest := makeSnapshot("fixture-latest", time.Date(2026, 9, 7, 12, 1, 0, 0, time.UTC), 12)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return plannerViewFixtureData{dbPath: dbPath, first: first, latest: latest}
}

func executePlannerViewJSON(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := RootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(append(args, "--json", "--no-learn"))
	err := root.Execute()
	if err != nil && errOut.Len() > 0 {
		err = fmt.Errorf("%w (stderr: %s)", err, strings.TrimSpace(errOut.String()))
	}
	return out.String(), err
}

func decodePlannerViewOutput(t *testing.T, data string, value any) {
	t.Helper()
	if err := decodeOutputJSON([]byte(data), value); err != nil {
		t.Fatalf("decode output: %v\n%s", err, data)
	}
}

func TestPlannerPortfolioViewsAreRegisteredAsLocalReadOnlyLeaves(t *testing.T) {
	root := RootCmd()
	if root.Use != "keyword-planner-pp-cli" {
		t.Fatalf("root.Use = %q, want keyword-planner-pp-cli", root.Use)
	}
	portfolioCommand := findKeywordRootCommand(root, "portfolio")
	if portfolioCommand == nil {
		t.Fatal("portfolio group is not reachable")
	}
	if portfolioCommand.Annotations["pp:parent-group"] != "true" {
		t.Fatalf("portfolio annotations = %#v; want parent-group marker", portfolioCommand.Annotations)
	}
	for _, name := range []string{"trace", "coverage", "variants", "diff", "integrity", "safe-stats"} {
		command := findKeywordRootCommand(portfolioCommand, name)
		if command == nil {
			t.Fatalf("portfolio %s is not reachable", name)
		}
		if command.Annotations["mcp:read-only"] != "true" || command.Annotations["pp:data-source"] != "local" {
			t.Fatalf("portfolio %s annotations = %#v; want local read-only", name, command.Annotations)
		}
		if command.Annotations["pp:typed-exit-codes"] != "0,3" {
			t.Fatalf("portfolio %s typed exits = %q; want 0,3", name, command.Annotations["pp:typed-exit-codes"])
		}
	}
}

func TestPlannerPortfolioViewsUseStoreReportsAndBoundRendering(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fixture := newPlannerViewFixture(t)

	selected, err := executePlannerViewJSON(t, "portfolio", "trace", "latest", "--db", fixture.dbPath, "--keyword", "beef steak", "--month", "2026-07", "--select", "snapshot,match_count")
	if err != nil {
		t.Fatalf("trace --select: %v", err)
	}
	var selectedFields map[string]json.RawMessage
	decodePlannerViewOutput(t, selected, &selectedFields)
	if _, ok := selectedFields["snapshot"]; !ok {
		t.Fatalf("trace --select omitted snapshot: %s", selected)
	}
	if _, ok := selectedFields["matches"]; ok {
		t.Fatalf("trace --select leaked unselected matches: %s", selected)
	}

	traceJSON, err := executePlannerViewJSON(t, "portfolio", "trace", "latest", "--db", fixture.dbPath, "--keyword", "beef steak", "--month", "2026-07")
	if err != nil {
		t.Fatalf("trace: %v", err)
	}
	var trace plannerTraceOutput
	decodePlannerViewOutput(t, traceJSON, &trace)
	if trace.MatchCount != 1 || len(trace.Matches) != 1 || trace.Resolution != "unique" || !trace.Matches[0].Receipt.BodyPresent {
		t.Fatalf("trace report = %#v", trace)
	}

	coverageJSON, err := executePlannerViewJSON(t, "portfolio", "coverage", "latest", "--db", fixture.dbPath, "--limit", "1")
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	var coverage plannerCoverageOutput
	decodePlannerViewOutput(t, coverageJSON, &coverage)
	if coverage.RequestedCount != 2 || coverage.ReturnedCount != 2 || len(coverage.Keywords) != 1 || len(coverage.Pages) != 1 || !coverage.CoverageTruncated || coverage.RenderedKeywordCount != 1 || coverage.RenderedPageCount != 1 {
		t.Fatalf("coverage report = %#v", coverage)
	}

	variantsJSON, err := executePlannerViewJSON(t, "portfolio", "variants", "latest", "--db", fixture.dbPath, "--keyword", "beef steak", "--limit", "1")
	if err != nil {
		t.Fatalf("variants: %v", err)
	}
	var variants plannerVariantsOutput
	decodePlannerViewOutput(t, variantsJSON, &variants)
	if variants.MetricCount != 2 || len(variants.Metrics) != 1 || len(variants.Links) > 1 || !variants.LinksTruncated || variants.RenderLimit != 1 {
		t.Fatalf("variants report = %#v", variants)
	}

	diffJSON, err := executePlannerViewJSON(t, "portfolio", "diff", "previous", "latest", "--db", fixture.dbPath)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	var diff plannerDiffOutput
	decodePlannerViewOutput(t, diffJSON, &diff)
	if !diff.Comparable || diff.Summary.ChangedRows == 0 || len(diff.Changes) == 0 {
		t.Fatalf("diff report = %#v", diff)
	}

	integrityJSON, err := executePlannerViewJSON(t, "portfolio", "integrity", "latest", "--db", fixture.dbPath)
	if err != nil {
		t.Fatalf("integrity: %v", err)
	}
	var integrity plannerIntegrityOutput
	decodePlannerViewOutput(t, integrityJSON, &integrity)
	if !integrity.Valid || integrity.ReceiptCount != 1 || len(integrity.Issues) != 0 {
		t.Fatalf("integrity report = %#v", integrity)
	}

	statsJSON, err := executePlannerViewJSON(t, "portfolio", "safe-stats", "--snapshot", "latest", "--db", fixture.dbPath, "--limit", "1")
	if err != nil {
		t.Fatalf("safe-stats: %v", err)
	}
	var stats plannerSafeStatsOutput
	decodePlannerViewOutput(t, statsJSON, &stats)
	if stats.RowsExamined != 2 || stats.RowsIncluded != 2 || len(stats.Groups) != 1 || !stats.GroupsTruncated || stats.Groups[0].SumMonthlySearches == "30" {
		t.Fatalf("safe-stats report = %#v", stats)
	}
}

func TestPlannerPortfolioViewsValidateBeforeIOAndRepresentEmptyOfflineState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	missingDB := filepath.Join(t.TempDir(), "missing.db")

	dryRun, err := executePlannerViewJSON(t, "portfolio", "coverage", "latest", "--db", missingDB, "--dry-run")
	if err != nil {
		t.Fatalf("coverage dry-run: %v", err)
	}
	var dryRunResult struct {
		DryRun bool   `json:"dry_run"`
		Action string `json:"action"`
	}
	decodePlannerViewOutput(t, dryRun, &dryRunResult)
	if !dryRunResult.DryRun || dryRunResult.Action != "portfolio coverage" {
		t.Fatalf("dry-run result = %#v", dryRunResult)
	}

	empty, err := executePlannerViewJSON(t, "portfolio", "coverage", "latest", "--db", missingDB)
	if err != nil {
		t.Fatalf("missing coverage: %v", err)
	}
	var emptyResult []json.RawMessage
	decodePlannerViewOutput(t, empty, &emptyResult)
	if emptyResult == nil || len(emptyResult) != 0 {
		t.Fatalf("missing portfolio coverage = %s", empty)
	}

	_, err = executePlannerViewJSON(t, "portfolio", "trace", "latest", "--db", missingDB, "--keyword", "beef steak", "--month", "2026-13")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("invalid trace month error = %v, exit=%d; want usage exit 2 before DB access", err, ExitCode(err))
	}

	_, err = executePlannerViewJSON(t, "portfolio", "safe-stats", "--db", missingDB, "--start", "2026-07")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("incomplete safe-stats range error = %v, exit=%d; want usage exit 2", err, ExitCode(err))
	}
}

func TestPlannerPortfolioViewsSupportStableCSVAndLiteralFilters(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fixture := newPlannerViewFixture(t)
	root := RootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"portfolio", "variants", "latest", "--db", fixture.dbPath, "--limit", "1", "--csv", "--no-learn"})
	if err := root.Execute(); err != nil {
		t.Fatalf("variants literal filter CSV: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("CSV output is empty: %q", out.String())
	}
	if !strings.HasPrefix(lines[0], "attribution_status,") {
		t.Fatalf("variants CSV header is not stable: %s", out.String())
	}

	filteredJSON, err := executePlannerViewJSON(t, "portfolio", "variants", "latest", "--db", fixture.dbPath, "--keyword", "beef%")
	if err != nil {
		t.Fatalf("variants literal filter: %v", err)
	}
	var filtered plannerVariantsOutput
	decodePlannerViewOutput(t, filteredJSON, &filtered)
	if len(filtered.Metrics) != 0 {
		t.Fatalf("literal wildcard filter unexpectedly matched metrics: %#v", filtered.Metrics)
	}

	root = RootCmd()
	out.Reset()
	errOut.Reset()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"portfolio", "safe-stats", "--snapshot", "latest", "--db", fixture.dbPath, "--csv", "--limit", "1", "--no-learn"})
	if err := root.Execute(); err != nil {
		t.Fatalf("safe-stats CSV: %v", err)
	}
	if !strings.Contains(out.String(), "keyword") || !strings.Contains(out.String(), "sum_monthly_searches") {
		t.Fatalf("safe-stats CSV header lacks stable group fields: %s", out.String())
	}
}

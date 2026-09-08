// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/portfolio"
	"github.com/spf13/cobra"
)

// The portfolio report types are owned by internal/portfolio. This file only
// supplies the curated Cobra surface, input validation, bounded rendering and
// typed errors for the six approved local evidence views.
var (
	plannerTraceFields = map[string]bool{
		"snapshot": true, "keyword": true, "month": true, "matches": true,
		"match_count": true, "ambiguous": true, "resolution": true,
		"warnings": true, "rendered_match_count": true, "render_limit": true,
		"matches_truncated": true,
		"row":               true, "metric": true, "receipt": true,
		"snapshot_id": true, "receipt_id": true, "metric_id": true,
		"endpoint": true, "page_token": true, "request_body": true,
		"request_body_present": true, "submitted_keywords": true, "http_status": true,
		"google_request_id": true, "raw_body_base64": true, "body_present": true,
		"fetched_at": true, "error_code": true, "error_message": true, "normalized": true,
		"returned_text": true, "submitted_text": true, "submitted_index": true,
		"variant_group": true, "linkage_status": true, "average_monthly_searches": true,
		"average_monthly_searches_state": true, "competition": true,
		"competition_index": true, "low_bid_micros": true, "high_bid_micros": true,
		"average_cpc_micros": true, "average_cpc_state": true, "annotations": true,
		"raw_result": true, "raw_result_present": true, "status": true, "flags": true,
		"currency": true, "currency_source": true, "geo_set_id": true,
		"geo_targets": true, "language": true, "network": true,
		"raw_month": true, "raw_year": true, "monthly_searches": true,
		"value_state": true, "requested_start": true, "requested_end": true, "complete": true,
	}
	plannerCoverageFields = map[string]bool{
		"snapshot": true, "keywords": true, "pages": true,
		"requested_terms": true, "returned_terms": true, "requested_months": true,
		"returned_months": true, "missing_months": true, "unavailable_months": true,
		"raw_only_months": true, "requested_count": true, "returned_count": true,
		"returned_month_count": true, "no_result_batches": true, "incomplete_pages": true,
		"short_window": true, "no_result": true, "complete": true, "status": true,
		"warnings": true, "rendered_keyword_count": true, "rendered_page_count": true,
		"render_limit": true, "coverage_truncated": true,
		"metric_id": true, "receipt_id": true, "endpoint": true, "page_number": true,
		"batch_number": true, "attempt": true, "returned_text": true,
		"submitted_text": true, "submitted_index": true, "variant_group": true,
		"linkage_status": true, "flags": true, "collection_status": true,
		"requested_start": true, "requested_end": true, "next_page_token": true,
		"vendor_total_size": true,
	}
	plannerVariantsFields = map[string]bool{
		"snapshot": true, "submitted": true, "submitted_seeds": true,
		"submitted_keywords": true, "metrics": true, "links": true,
		"canonical_terms": true, "metric_count": true, "monthly_row_count": true,
		"link_count": true, "exact_count": true, "normalized_count": true,
		"close_variant_count": true, "unmatched_count": true, "unknown_count": true,
		"combined_request": true, "attribution_status": true, "warnings": true,
		"rendered_metric_count": true, "rendered_link_count": true, "render_limit": true,
		"metrics_truncated": true, "links_truncated": true,
		"id": true, "snapshot_id": true, "receipt_id": true, "metric_id": true,
		"endpoint": true, "returned_text": true, "canonical_text": true,
		"submitted_text": true, "submitted_index": true, "variant_group": true,
		"linkage_status": true, "flags": true, "rows": true,
		"created_at": true, "relationship": true,
	}
	plannerDiffFields = map[string]bool{
		"left_id": true, "right_id": true, "left": true, "right": true,
		"comparable": true, "incompatibilities": true, "request_changes": true,
		"changes": true, "coverage_changes": true, "summary": true,
		"range_changed": true, "range_change_warning": true,
		"rendered_change_count": true, "rendered_coverage_change_count": true,
		"render_limit": true, "changes_truncated": true,
		"coverage_changes_truncated": true,
		"field":                      true, "kind": true, "key": true,
		"changed_fields": true, "reason": true, "scope_note": true,
		"endpoint": true, "page_number": true, "batch_number": true, "attempt": true,
	}
	plannerIntegrityFields = map[string]bool{
		"snapshot": true, "valid": true, "checks": true, "issues": true,
		"receipt_count": true, "metric_count": true, "monthly_count": true,
		"coverage_count": true, "variant_count": true, "account_event_count": true,
		"rendered_issue_count": true, "render_limit": true, "issues_truncated": true,
		"name": true, "passed": true, "issue_count": true, "check": true,
		"table": true, "id": true, "detail": true,
	}
	plannerSafeStatsFields = map[string]bool{
		"snapshot": true, "groups": true, "rows_examined": true, "rows_included": true,
		"excluded_rows": true, "excluded_incomplete": true, "excluded_null": true,
		"excluded_invalid": true, "excluded_negative": true,
		"excluded_ambiguous_zero_unspecified": true, "excluded_raw_only": true,
		"warnings": true, "no_eligible_values": true, "filters": true,
		"rendered_group_count": true, "render_limit": true, "groups_truncated": true,
		"snapshot_id": true, "metric_id": true, "keyword": true, "submitted_keyword": true,
		"submitted_index": true, "variant_group": true, "linkage_status": true,
		"endpoint": true, "geo_set_id": true, "geo_targets": true, "language": true,
		"network": true, "currency": true, "currency_source": true,
		"requested_start": true, "requested_end": true, "requested_months": true,
		"returned_months": true, "observed_months": true, "null_rows": true,
		"short_window": true, "combined_geo": true, "sum_monthly_searches": true,
		"observed_mean": true, "min_monthly_searches": true, "max_monthly_searches": true,
		"flags": true,
	}
)

type plannerTraceOutput struct {
	portfolio.TraceReport
	RenderedMatchCount int  `json:"rendered_match_count"`
	RenderLimit        int  `json:"render_limit"`
	MatchesTruncated   bool `json:"matches_truncated"`
}

type plannerCoverageOutput struct {
	portfolio.CoverageReport
	RenderedKeywordCount int  `json:"rendered_keyword_count"`
	RenderedPageCount    int  `json:"rendered_page_count"`
	RenderLimit          int  `json:"render_limit"`
	CoverageTruncated    bool `json:"coverage_truncated"`
}

type plannerVariantsOutput struct {
	portfolio.VariantsReport
	RenderedMetricCount int  `json:"rendered_metric_count"`
	RenderedLinkCount   int  `json:"rendered_link_count"`
	RenderLimit         int  `json:"render_limit"`
	MetricsTruncated    bool `json:"metrics_truncated"`
	LinksTruncated      bool `json:"links_truncated"`
}

type plannerDiffOutput struct {
	portfolio.SnapshotDiff
	RenderedChangeCount         int  `json:"rendered_change_count"`
	RenderedCoverageChangeCount int  `json:"rendered_coverage_change_count"`
	RenderLimit                 int  `json:"render_limit"`
	ChangesTruncated            bool `json:"changes_truncated"`
	CoverageChangesTruncated    bool `json:"coverage_changes_truncated"`
}

type plannerIntegrityOutput struct {
	portfolio.IntegrityResult
	RenderedIssueCount int  `json:"rendered_issue_count"`
	RenderLimit        int  `json:"render_limit"`
	IssuesTruncated    bool `json:"issues_truncated"`
}

type plannerSafeStatsOutput struct {
	portfolio.SafeStatsResult
	Snapshot           portfolio.Snapshot         `json:"snapshot"`
	Filters            portfolio.SafeStatsOptions `json:"filters"`
	RenderedGroupCount int                        `json:"rendered_group_count"`
	RenderLimit        int                        `json:"render_limit"`
	GroupsTruncated    bool                       `json:"groups_truncated"`
}

func bindPlannerViewDB(cmd *cobra.Command, destination *string) {
	cmd.Flags().StringVar(destination, "db", "", "Portfolio SQLite path (env KEYWORD_PLANNER_DB is used when omitted)")
}

func bindPlannerViewSnapshot(cmd *cobra.Command, destination *string) {
	cmd.Flags().StringVar(destination, "snapshot", "", "Snapshot ID or alias latest/previous")
}

func bindPlannerViewLimit(cmd *cobra.Command, destination *int, description string) {
	cmd.Flags().IntVar(destination, "limit", 0, description)
}

func plannerViewHelpOnly(cmd *cobra.Command, args []string) bool {
	if cmd == nil || len(args) != 0 {
		return false
	}
	return cmd.Flags().NFlag() == 0 && cmd.InheritedFlags().NFlag() == 0 && cmd.PersistentFlags().NFlag() == 0
}

func plannerViewAlias(args []string, flagValue string, command string) (string, error) {
	if len(args) > 1 {
		return "", fmt.Errorf("%s accepts at most one snapshot ID or alias", command)
	}
	alias := strings.TrimSpace(flagValue)
	if len(args) == 1 {
		argument := strings.TrimSpace(args[0])
		if argument == "" {
			return "", errors.New("snapshot ID or alias cannot be empty")
		}
		if alias != "" && alias != argument {
			return "", errors.New("snapshot may be supplied as an argument or --snapshot, not both")
		}
		alias = argument
	}
	if alias == "" {
		return "", errors.New("provide a snapshot ID or alias latest/previous")
	}
	return alias, nil
}

func plannerViewLimit(limit int) error {
	if limit < 0 {
		return fmt.Errorf("limit cannot be negative; got %d", limit)
	}
	return nil
}

func plannerViewError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, portfolio.ErrNotFound) {
		return notFoundErr(err)
	}
	if errors.Is(err, portfolio.ErrInvalidInput) {
		return usageErr(err)
	}
	return configErr(err)
}

func plannerViewContext(cmd *cobra.Command, flags *rootFlags) (context.Context, context.CancelFunc) {
	return boundCtx(cmd.Context(), flags)
}

func newPlannerPortfolioTraceCmd(flags *rootFlags) *cobra.Command {
	var query struct {
		dbPath   string
		snapshot string
		keyword  string
		limit    int
	}
	var month string
	cmd := &cobra.Command{
		Use:   "trace [snapshot-id]",
		Short: "Trace one stored monthly value to its exact request and raw receipt.",
		Long:  "Use this command to trace one stored monthly value to its immutable raw response receipt. Do NOT use it to compare two collections; use portfolio diff instead.",
		Example: strings.Trim(`
  keyword-planner portfolio trace 2c2f465c-6253-430a-a533-aa30fabbfbff --keyword "beef steak" --month 2026-07 --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true", "pp:data-source": "local",
			"pp:typed-exit-codes": "0,3",
			"pp:happy-args":       "snapshot=2c2f465c-6253-430a-a533-aa30fabbfbff;--keyword=beef steak;--month=2026-07",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if plannerViewHelpOnly(cmd, args) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "portfolio trace")
			}
			if err := rejectPlannerEvidenceBypass(flags); err != nil {
				return usageErr(err)
			}
			alias, err := plannerViewAlias(args, query.snapshot, "trace")
			if err != nil {
				return usageErr(err)
			}
			keyword := strings.TrimSpace(query.keyword)
			if keyword == "" {
				return usageErr(errors.New("--keyword is required to identify one returned term"))
			}
			parsedMonth, err := portfolio.ParseYearMonth(month)
			if err != nil {
				return usageErr(err)
			}
			if err := plannerViewLimit(query.limit); err != nil {
				return usageErr(err)
			}
			ctx, cancel := plannerViewContext(cmd, flags)
			defer cancel()
			store, missing, err := openPlannerPortfolioReadOnly(ctx, plannerPortfolioDBPath(query.dbPath, plannerRootHome(flags)))
			if err != nil {
				return plannerViewError(err)
			}
			if missing {
				return notFoundErr(fmt.Errorf("portfolio database is missing; no stored value exists for snapshot %q", alias))
			}
			defer store.Close()
			report, err := store.Trace(ctx, alias, portfolio.TraceOptions{Keyword: keyword, Month: parsedMonth.String(), IncludeRaw: true})
			if err != nil {
				return plannerViewError(err)
			}
			report = filterPlannerTraceExact(report, keyword)
			output := plannerTraceOutput{TraceReport: report, RenderedMatchCount: len(report.Matches), RenderLimit: query.limit}
			if query.limit > 0 && len(output.Matches) > query.limit {
				output.Matches = append([]portfolio.TraceMatch(nil), output.Matches[:query.limit]...)
				output.RenderedMatchCount = len(output.Matches)
				output.MatchesTruncated = true
			}
			if output.Matches == nil {
				output.Matches = []portfolio.TraceMatch{}
			}
			if err := writePlannerLocalOutput(cmd, flags, output, plannerTraceFields); err != nil {
				return err
			}
			if report.MatchCount == 0 {
				return notFoundErr(fmt.Errorf("no normalized monthly value for keyword %q in %s", keyword, parsedMonth.String()))
			}
			return nil
		},
	}
	bindPlannerViewDB(cmd, &query.dbPath)
	bindPlannerViewSnapshot(cmd, &query.snapshot)
	cmd.Flags().StringVar(&query.keyword, "keyword", "", "Exact returned or submitted keyword to trace")
	cmd.Flags().StringVar(&month, "month", "", "Closed month to trace, in YYYY-MM form")
	bindPlannerViewLimit(cmd, &query.limit, "Maximum matching values to render; zero means all")
	return cmd
}

func newPlannerPortfolioCoverageCmd(flags *rootFlags) *cobra.Command {
	var query struct {
		dbPath   string
		snapshot string
		keyword  string
		limit    int
	}
	cmd := &cobra.Command{
		Use:   "coverage [snapshot-id]",
		Short: "Inspect requested, returned, missing, unavailable, raw-only, and incomplete coverage.",
		Long:  "Use this command to inspect requested and returned coverage for one snapshot. Do NOT use it to trace an individual metric to its raw body; use portfolio trace instead.",
		Example: strings.Trim(`
  keyword-planner portfolio coverage latest --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true", "pp:data-source": "local",
			"pp:typed-exit-codes": "0,3",
			"pp:happy-args":       "snapshot=latest",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if plannerViewHelpOnly(cmd, args) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "portfolio coverage")
			}
			if err := rejectPlannerEvidenceBypass(flags); err != nil {
				return usageErr(err)
			}
			alias, err := plannerViewAlias(args, query.snapshot, "coverage")
			if err != nil {
				return usageErr(err)
			}
			if err := plannerViewLimit(query.limit); err != nil {
				return usageErr(err)
			}
			ctx, cancel := plannerViewContext(cmd, flags)
			defer cancel()
			store, missing, err := openPlannerPortfolioReadOnly(ctx, plannerPortfolioDBPath(query.dbPath, plannerRootHome(flags)))
			if err != nil {
				return plannerViewError(err)
			}
			if missing {
				return writePlannerLocalRaw(cmd, flags, []byte("[]"), plannerCoverageFields)
			}
			defer store.Close()
			report, err := store.CoverageReport(ctx, alias, portfolio.CoverageOptions{Keyword: strings.TrimSpace(query.keyword)})
			if err != nil {
				return plannerViewError(err)
			}
			output := plannerCoverageOutput{CoverageReport: report, RenderedKeywordCount: len(report.Keywords), RenderedPageCount: len(report.Pages), RenderLimit: query.limit}
			if query.limit > 0 {
				if len(output.Keywords) > query.limit {
					output.Keywords = append([]portfolio.KeywordCoverage(nil), output.Keywords[:query.limit]...)
					output.CoverageTruncated = true
				}
				if len(output.Pages) > query.limit {
					output.Pages = append([]portfolio.CoverageView(nil), output.Pages[:query.limit]...)
					output.CoverageTruncated = true
				}
			}
			output.RenderedKeywordCount = len(output.Keywords)
			output.RenderedPageCount = len(output.Pages)
			if output.Keywords == nil {
				output.Keywords = []portfolio.KeywordCoverage{}
			}
			if output.Pages == nil {
				output.Pages = []portfolio.CoverageView{}
			}
			return writePlannerLocalOutput(cmd, flags, output, plannerCoverageFields)
		},
	}
	bindPlannerViewDB(cmd, &query.dbPath)
	bindPlannerViewSnapshot(cmd, &query.snapshot)
	cmd.Flags().StringVar(&query.keyword, "keyword", "", "Optional literal keyword filter for coverage")
	bindPlannerViewLimit(cmd, &query.limit, "Maximum coverage keywords and pages to render; zero means all")
	return cmd
}

func newPlannerPortfolioVariantsCmd(flags *rootFlags) *cobra.Command {
	var query struct {
		dbPath   string
		snapshot string
		keyword  string
		limit    int
	}
	cmd := &cobra.Command{
		Use:   "variants [snapshot-id]",
		Short: "Inspect submitted-to-returned keyword and close-variant lineage without double-counting demand.",
		Long:  "Use this command to inspect submitted-to-returned keyword and close-variant linkage. Do NOT use it to compare snapshots; use portfolio diff instead.",
		Example: strings.Trim(`
  keyword-planner portfolio variants latest --limit 10 --agent --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true", "pp:data-source": "local",
			"pp:typed-exit-codes": "0,3",
			"pp:happy-args":       "snapshot=latest",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if plannerViewHelpOnly(cmd, args) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "portfolio variants")
			}
			if err := rejectPlannerEvidenceBypass(flags); err != nil {
				return usageErr(err)
			}
			alias, err := plannerViewAlias(args, query.snapshot, "variants")
			if err != nil {
				return usageErr(err)
			}
			if err := plannerViewLimit(query.limit); err != nil {
				return usageErr(err)
			}
			ctx, cancel := plannerViewContext(cmd, flags)
			defer cancel()
			store, missing, err := openPlannerPortfolioReadOnly(ctx, plannerPortfolioDBPath(query.dbPath, plannerRootHome(flags)))
			if err != nil {
				return plannerViewError(err)
			}
			if missing {
				return writePlannerLocalRaw(cmd, flags, []byte("[]"), plannerVariantsFields)
			}
			defer store.Close()
			report, err := store.VariantsReport(ctx, alias, portfolio.VariantsOptions{Keyword: strings.TrimSpace(query.keyword)})
			if err != nil {
				return plannerViewError(err)
			}
			output := plannerVariantsOutput{VariantsReport: report, RenderedMetricCount: len(report.Metrics), RenderedLinkCount: len(report.Links), RenderLimit: query.limit}
			if query.limit > 0 {
				if len(output.Metrics) > query.limit {
					output.Metrics = append([]portfolio.VariantMetric(nil), output.Metrics[:query.limit]...)
					output.MetricsTruncated = true
				}
				if len(output.Links) > query.limit {
					output.Links = append([]portfolio.VariantLink(nil), output.Links[:query.limit]...)
					output.LinksTruncated = true
				}
			}
			output.RenderedMetricCount = len(output.Metrics)
			output.RenderedLinkCount = len(output.Links)
			if output.Metrics == nil {
				output.Metrics = []portfolio.VariantMetric{}
			}
			if output.Links == nil {
				output.Links = []portfolio.VariantLink{}
			}
			return writePlannerLocalOutput(cmd, flags, output, plannerVariantsFields)
		},
	}
	bindPlannerViewDB(cmd, &query.dbPath)
	cmd.Flags().StringVar(&query.snapshot, "snapshot", "", "Snapshot ID or alias latest/previous")
	cmd.Flags().StringVar(&query.keyword, "keyword", "", "Literal keyword filter for lineage")
	cmd.Flags().IntVar(&query.limit, "limit", 0, "Maximum metric and link records to render; zero means all")
	return cmd
}

func newPlannerPortfolioDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "diff [left-snapshot-id] [right-snapshot-id]",
		Short: "Compare two immutable snapshots by request scope, monthly observations, and coverage.",
		Long:  "Use this command to compare two immutable snapshots. Do NOT use it to inspect one snapshot's missing months or page completeness; use portfolio coverage instead.",
		Example: strings.Trim(`
  keyword-planner portfolio diff 2c2f465c-6253-430a-a533-aa30fabbfbff c6ea4780-190b-42f4-8f4d-a8b7207a2395 --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true", "pp:data-source": "local",
			"pp:typed-exit-codes": "0,3",
			"pp:happy-args":       "left=2c2f465c-6253-430a-a533-aa30fabbfbff;right=c6ea4780-190b-42f4-8f4d-a8b7207a2395",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if plannerViewHelpOnly(cmd, args) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "portfolio diff")
			}
			if len(args) != 2 {
				return usageErr(errors.New("diff requires two snapshot IDs or aliases: <left> <right>"))
			}
			leftAlias, rightAlias := strings.TrimSpace(args[0]), strings.TrimSpace(args[1])
			if leftAlias == "" || rightAlias == "" {
				return usageErr(errors.New("left and right snapshot IDs or aliases cannot be empty"))
			}
			if err := plannerViewLimit(limit); err != nil {
				return usageErr(err)
			}
			if err := rejectPlannerEvidenceBypass(flags); err != nil {
				return usageErr(err)
			}
			ctx, cancel := plannerViewContext(cmd, flags)
			defer cancel()
			store, missing, err := openPlannerPortfolioReadOnly(ctx, plannerPortfolioDBPath(dbPath, plannerRootHome(flags)))
			if err != nil {
				return plannerViewError(err)
			}
			if missing {
				return notFoundErr(fmt.Errorf("portfolio database is missing; snapshots %q and %q cannot be compared", leftAlias, rightAlias))
			}
			defer store.Close()
			report, diffErr := store.Diff(ctx, leftAlias, rightAlias)
			if diffErr != nil && errors.Is(diffErr, portfolio.ErrNotFound) {
				return notFoundErr(diffErr)
			}
			output := plannerDiffOutput{SnapshotDiff: report, RenderLimit: limit}
			if limit > 0 {
				if len(output.Changes) > limit {
					output.Changes = append([]portfolio.DiffChange(nil), output.Changes[:limit]...)
					output.ChangesTruncated = true
				}
				if len(output.CoverageChanges) > limit {
					output.CoverageChanges = append([]portfolio.CoverageDiff(nil), output.CoverageChanges[:limit]...)
					output.CoverageChangesTruncated = true
				}
			}
			output.RenderedChangeCount = len(output.Changes)
			output.RenderedCoverageChangeCount = len(output.CoverageChanges)
			if output.Changes == nil {
				output.Changes = []portfolio.DiffChange{}
			}
			if output.CoverageChanges == nil {
				output.CoverageChanges = []portfolio.CoverageDiff{}
			}
			if err := writePlannerLocalOutput(cmd, flags, output, plannerDiffFields); err != nil {
				return err
			}
			if diffErr != nil {
				return plannerViewError(diffErr)
			}
			return nil
		},
	}
	bindPlannerViewDB(cmd, &dbPath)
	bindPlannerViewLimit(cmd, &limit, "Maximum row and coverage changes to render; zero means all")
	return cmd
}

func newPlannerPortfolioIntegrityCmd(flags *rootFlags) *cobra.Command {
	var dbPath, snapshot string
	var limit int
	cmd := &cobra.Command{
		Use:   "integrity [snapshot-id]",
		Short: "Verify receipt hashes and normalized-row links for one immutable snapshot.",
		Long:  "Use this command to verify receipt hashes and normalized-row links for one immutable snapshot. Do NOT use it to inspect request coverage; use portfolio coverage instead.",
		Example: strings.Trim(`
  keyword-planner portfolio integrity latest --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true", "pp:data-source": "local",
			"pp:typed-exit-codes": "0,3",
			"pp:happy-args":       "snapshot=latest",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if plannerViewHelpOnly(cmd, args) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "portfolio integrity")
			}
			if err := rejectPlannerEvidenceBypass(flags); err != nil {
				return usageErr(err)
			}
			alias, err := plannerViewAlias(args, snapshot, "integrity")
			if err != nil {
				return usageErr(err)
			}
			if err := plannerViewLimit(limit); err != nil {
				return usageErr(err)
			}
			ctx, cancel := plannerViewContext(cmd, flags)
			defer cancel()
			store, missing, err := openPlannerPortfolioReadOnly(ctx, plannerPortfolioDBPath(dbPath, plannerRootHome(flags)))
			if err != nil {
				return plannerViewError(err)
			}
			if missing {
				return notFoundErr(fmt.Errorf("portfolio database is missing; snapshot %q has no evidence", alias))
			}
			defer store.Close()
			report, integrityErr := store.Integrity(ctx, alias)
			if integrityErr != nil && errors.Is(integrityErr, portfolio.ErrNotFound) {
				return notFoundErr(integrityErr)
			}
			output := plannerIntegrityOutput{IntegrityResult: report, RenderLimit: limit, RenderedIssueCount: len(report.Issues)}
			if limit > 0 && len(output.Issues) > limit {
				output.Issues = append([]portfolio.IntegrityIssue(nil), output.Issues[:limit]...)
				output.RenderedIssueCount = len(output.Issues)
				output.IssuesTruncated = true
			}
			if output.Issues == nil {
				output.Issues = []portfolio.IntegrityIssue{}
			}
			if err := writePlannerLocalOutput(cmd, flags, output, plannerIntegrityFields); err != nil {
				return err
			}
			if integrityErr != nil {
				return plannerViewError(integrityErr)
			}
			return nil
		},
	}
	bindPlannerViewDB(cmd, &dbPath)
	bindPlannerViewSnapshot(cmd, &snapshot)
	bindPlannerViewLimit(cmd, &limit, "Maximum integrity issues to render; zero means all")
	return cmd
}

func newPlannerPortfolioSafeStatsCmd(flags *rootFlags) *cobra.Command {
	var query plannerPortfolioQueryFlags
	cmd := &cobra.Command{
		Use:   "safe-stats",
		Short: "Calculate descriptive aggregates from eligible local monthly rows.",
		Long:  "Use this command to calculate policy-safe aggregates from eligible local monthly rows. Do NOT use it to trace a displayed value to raw evidence; use portfolio trace instead.",
		Example: strings.Trim(`
  keyword-planner portfolio safe-stats --snapshot latest --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true", "pp:data-source": "local",
			"pp:typed-exit-codes": "0,3",
			"pp:happy-args":       "--snapshot=latest",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if plannerViewHelpOnly(cmd, args) {
				return cmd.Help()
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("safe-stats does not accept positional arguments; use --snapshot"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "portfolio safe-stats")
			}
			if err := rejectPlannerEvidenceBypass(flags); err != nil {
				return usageErr(err)
			}
			if strings.TrimSpace(query.snapshot) == "" {
				query.snapshot = "latest"
			}
			if err := plannerViewLimit(query.limit); err != nil {
				return usageErr(err)
			}
			options, err := plannerPortfolioOptions(query)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := plannerViewContext(cmd, flags)
			defer cancel()
			store, missing, err := openPlannerPortfolioReadOnly(ctx, plannerPortfolioDBPath(query.dbPath, plannerRootHome(flags)))
			if err != nil {
				return plannerViewError(err)
			}
			if missing {
				return writePlannerLocalRaw(cmd, flags, []byte("[]"), plannerSafeStatsFields)
			}
			defer store.Close()
			report, statsErr := store.SafeStats(ctx, portfolio.SafeStatsOptions{
				SnapshotID: query.snapshot, Keyword: options.Keyword, StartMonth: options.StartMonth,
				EndMonth: options.EndMonth, Language: options.Language, GeoSetID: options.GeoSetID,
				GeoTargets: options.GeoTargets, Endpoint: options.Endpoint,
			})
			if statsErr != nil {
				return plannerViewError(statsErr)
			}
			items, err := store.ListSnapshots(ctx, portfolio.QueryOptions{SnapshotID: query.snapshot, Limit: 1})
			if err != nil {
				return plannerViewError(err)
			}
			if len(items) == 0 {
				return notFoundErr(portfolio.ErrNotFound)
			}
			output := plannerSafeStatsOutput{
				SafeStatsResult: report, Snapshot: items[0], Filters: portfolio.SafeStatsOptions{
					SnapshotID: query.snapshot, Keyword: options.Keyword, StartMonth: options.StartMonth,
					EndMonth: options.EndMonth, Language: options.Language, GeoSetID: options.GeoSetID,
					GeoTargets: options.GeoTargets, Endpoint: options.Endpoint,
				},
				RenderedGroupCount: len(report.Groups), RenderLimit: query.limit,
			}
			if query.limit > 0 && len(output.Groups) > query.limit {
				output.Groups = append([]portfolio.SafeStatsGroup(nil), output.Groups[:query.limit]...)
				output.RenderedGroupCount = len(output.Groups)
				output.GroupsTruncated = true
			}
			if output.Groups == nil {
				output.Groups = []portfolio.SafeStatsGroup{}
			}
			return writePlannerLocalOutput(cmd, flags, output, plannerSafeStatsFields)
		},
	}
	bindPlannerViewDB(cmd, &query.dbPath)
	cmd.Flags().StringVar(&query.snapshot, "snapshot", "", "Snapshot ID or alias latest/previous")
	cmd.Flags().StringVar(&query.keyword, "keyword", "", "Literal keyword filter for eligible rows")
	cmd.Flags().StringVar(&query.language, "language", "", "Language resource filter, for example languageConstants/1000")
	cmd.Flags().StringArrayVar(&query.geoTargets, "geo", nil, "Exact geo target resource filter; repeat for multiple targets")
	cmd.Flags().StringVar(&query.endpoint, "endpoint", "", "Collection endpoint filter: ideas or historical")
	cmd.Flags().StringVar(&query.start, "start", "", "First month filter, YYYY-MM")
	cmd.Flags().StringVar(&query.end, "end", "", "Last month filter, YYYY-MM")
	cmd.Flags().IntVar(&query.limit, "limit", 0, "Maximum metric groups to render; zero means all")
	return cmd
}

// registerKeywordPlannerViews is called by the curated registration hook after
// list/show/search/export are attached. addNovelCommandIfAbsent replaces each
// generated scaffold with the preserved command implementation here.
func registerKeywordPlannerViews(portfolioCommand *cobra.Command, flags *rootFlags) {
	if portfolioCommand == nil {
		return
	}
	addNovelCommandIfAbsent(portfolioCommand, newPlannerPortfolioTraceCmd(flags))
	addNovelCommandIfAbsent(portfolioCommand, newPlannerPortfolioCoverageCmd(flags))
	addNovelCommandIfAbsent(portfolioCommand, newPlannerPortfolioVariantsCmd(flags))
	addNovelCommandIfAbsent(portfolioCommand, newPlannerPortfolioDiffCmd(flags))
	addNovelCommandIfAbsent(portfolioCommand, newPlannerPortfolioIntegrityCmd(flags))
	addNovelCommandIfAbsent(portfolioCommand, newPlannerPortfolioSafeStatsCmd(flags))
}

func filterPlannerTraceExact(report portfolio.TraceReport, keyword string) portfolio.TraceReport {
	want := strings.ToLower(strings.TrimSpace(keyword))
	matches := make([]portfolio.TraceMatch, 0, len(report.Matches))
	for _, match := range report.Matches {
		if strings.ToLower(strings.TrimSpace(match.Row.Keyword)) == want || strings.ToLower(strings.TrimSpace(match.Row.SubmittedKeyword)) == want || strings.ToLower(strings.TrimSpace(match.Metric.ReturnedText)) == want || strings.ToLower(strings.TrimSpace(match.Metric.SubmittedText)) == want {
			matches = append(matches, match)
		}
	}
	report.Matches = matches
	report.MatchCount = len(matches)
	report.Ambiguous = len(matches) > 1
	report.Resolution = "none"
	if len(matches) == 1 {
		report.Resolution = "unique"
	} else if len(matches) > 1 {
		report.Resolution = "ambiguous"
	}
	warnings := append([]string(nil), report.Warnings...)
	for _, match := range matches {
		if match.Row.MonthlySearches == nil {
			warnings = append(warnings, "monthly_value_unavailable")
		}
		if containsPlannerString(match.Row.Flags, "ambiguous_zero_unspecified") {
			warnings = append(warnings, "ambiguous_zero_unspecified")
		}
	}
	if len(matches) == 0 {
		warnings = append(warnings, "no_normalized_monthly_match")
	} else if len(matches) > 1 {
		warnings = append(warnings, "multiple_normalized_monthly_matches")
	}
	report.Warnings = uniquePlannerStrings(warnings)
	return report
}

func containsPlannerString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func uniquePlannerStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

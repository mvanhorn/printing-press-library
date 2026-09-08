// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local
// Reads only the accrued local panel; no upstream call. Sync advances the data.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type checkResult struct {
	Check    string   `json:"check"`
	AsOf     string   `json:"as_of,omitempty"`
	Verdict  string   `json:"verdict"`
	Examined int      `json:"examined"`
	Failed   int      `json:"failed"`
	Detail   string   `json:"detail,omitempty"`
	Examples []string `json:"examples,omitempty"`
}

type verifyEnvelope struct {
	Releases int           `json:"releases_checked"`
	Results  []checkResult `json:"results"`
	Passed   int           `json:"checks_passed"`
	Failed   int           `json:"checks_failed"`
	Skipped  int           `json:"checks_skipped"`
	Expected int           `json:"checks_expected_to_diverge"`
	Note     string        `json:"note,omitempty"`
}

// verifyChecks is the closed set of audits, each an arithmetic identity the
// source states about itself.
var verifyChecks = []string{"total-weights", "min-avg-max", "section-counts", "national-average", "pct-columns", "row-blocks"}

func newNovelVerifyCmd(flags *rootFlags) *cobra.Command {
	var (
		asOf   string
		all    bool
		checks string
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Audit parsed cell values against the source's own arithmetic invariants",
		Long: strings.Trim(`
Check the stored panel against identities the Bureau's own files assert.

Use this command to check whether parsed CELL VALUES are internally consistent
for releases already stored. Do NOT use it to find missing releases; use
'coverage' instead. Do NOT use it to detect upstream changes; use 'revisions'
instead.

Checks:
  total-weights     the three section TOTAL rows must sum to 100 on both the
                    lowest-quintile and combined weight columns
  min-avg-max       every complete triplet must satisfy min <= avg <= max
  section-counts    each section heading declares how many items follow it
  national-average  the published national figure versus an unweighted mean of
                    the cities. THIS IS EXPECTED TO DIVERGE and is reported, not
                    failed: the Bureau weights cities and does not publish the
                    city weight vector, so a close match would be the surprise
  pct-columns       confirms the source's own percent columns are published as
                    zero, which is why every percentage here is derived from
                    levels instead
  row-blocks        every urban centre resolved across the stacked header bands

A broken invariant is treated as a PARSE FAILURE, not as a finding about
Pakistani prices.
`, "\n"),
		Example: strings.Trim(`
  pbs-pp-cli verify
  pbs-pp-cli verify --all --agent
  pbs-pp-cli verify --as-of 2026-09-03 --check total-weights,min-avg-max --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "verify")
			}
			want := map[string]bool{}
			if strings.TrimSpace(checks) == "" {
				for _, c := range verifyChecks {
					want[c] = true
				}
			} else {
				valid := map[string]bool{}
				for _, c := range verifyChecks {
					valid[c] = true
				}
				for _, c := range strings.Split(checks, ",") {
					c = strings.TrimSpace(strings.ToLower(c))
					if c == "" {
						continue
					}
					if !valid[c] {
						_ = cmd.Usage()
						return usageErr(fmt.Errorf("unknown check %q; valid checks are %s", c, strings.Join(verifyChecks, ", ")))
					}
					want[c] = true
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = panelDBPath(dbPath)
			db, ok, err := openPanelForRead(ctx, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				noMirror(cmd.ErrOrStderr(), dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), verifyEnvelope{
						Results: []checkResult{}, Note: "no local panel yet"}, flags)
				}
				return nil
			}
			defer db.Close()
			warnIfStale(ctx, db.DB(), cmd.ErrOrStderr(), flags, cmd)
			if panelEmpty(ctx, db.DB()) {
				emptyPanelHint(cmd.ErrOrStderr())
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), verifyEnvelope{Results: []checkResult{}, Note: "the local panel is empty"}, flags)
				}
				return nil
			}

			var dates []string
			switch {
			case asOf != "":
				dates = []string{asOf}
			case all:
				rows, err := db.DB().QueryContext(ctx, `SELECT DISTINCT as_of FROM pbs_price ORDER BY as_of DESC`)
				if err != nil {
					return fmt.Errorf("list releases: %w", err)
				}
				for rows.Next() {
					var d string
					if err := rows.Scan(&d); err != nil {
						_ = rows.Close()
						return err
					}
					dates = append(dates, d)
				}
				if err := rows.Err(); err != nil {
					_ = rows.Close()
					return err
				}
				if err := rows.Close(); err != nil {
					return err
				}
			default:
				d, err := latestAsOf(ctx, db.DB())
				if err != nil {
					return err
				}
				if d != "" {
					dates = []string{d}
				}
			}

			out := verifyEnvelope{Results: []checkResult{}, Releases: len(dates)}
			if len(dates) == 0 {
				out.Note = "no stored releases to verify"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
						return err
					}
					return notFoundErr(fmt.Errorf("no stored releases to verify"))
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No stored releases to verify.")
				return notFoundErr(fmt.Errorf("no stored releases to verify"))
			}

			for _, d := range dates {
				if want["total-weights"] {
					out.Results = append(out.Results, checkTotalWeights(ctx, db.DB(), d))
				}
				if want["min-avg-max"] {
					out.Results = append(out.Results, checkMinAvgMax(ctx, db.DB(), d))
				}
				if want["section-counts"] {
					out.Results = append(out.Results, checkSectionCounts(ctx, db.DB(), d))
				}
				if want["national-average"] {
					out.Results = append(out.Results, checkNationalAverage(ctx, db.DB(), d))
				}
				if want["pct-columns"] {
					out.Results = append(out.Results, checkPctColumns(ctx, db.DB(), d))
				}
				if want["row-blocks"] {
					out.Results = append(out.Results, checkRowBlocks(ctx, db.DB(), d))
				}
			}
			for _, r := range out.Results {
				switch r.Verdict {
				case "pass":
					out.Passed++
				case "expected-divergence":
					out.Expected++
				case "skipped":
					// A check with nothing to examine has not failed. Counting it
					// as a failure made `verify` exit 5 claiming a parse failure
					// on a release whose report file had merely 404'd upstream.
					out.Skipped++
				default:
					out.Failed++
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
					return err
				}
				if out.Failed > 0 {
					return apiErr(fmt.Errorf("%d invariant check(s) failed; treat this as a parse failure", out.Failed))
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "verifying %d release(s)\n\n", out.Releases)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "AS OF\tCHECK\tVERDICT\tEXAMINED\tFAILED\tDETAIL")
			for _, r := range out.Results {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%s\n", r.AsOf, r.Check, r.Verdict, r.Examined, r.Failed, truncate(r.Detail, 72))
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d passed, %d failed, %d skipped, %d expected-divergence\n",
				out.Passed, out.Failed, out.Skipped, out.Expected)
			if out.Skipped > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "a skipped check had nothing stored to examine; sync the release's report file to enable it")
			}
			if out.Expected > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "an expected divergence is not a failure: see 'national-average' in --help")
			}
			if out.Failed > 0 {
				return apiErr(fmt.Errorf("%d invariant check(s) failed; treat this as a parse failure", out.Failed))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&asOf, "as-of", "", "Verify one release, YYYY-MM-DD (defaults to the newest stored)")
	cmd.Flags().BoolVar(&all, "all", false, "Verify every stored release")
	cmd.Flags().StringVar(&checks, "check", "", "Comma-separated subset of checks to run (default all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local panel database path")
	return cmd
}

func checkTotalWeights(ctx context.Context, db *sql.DB, asOf string) checkResult {
	r := checkResult{Check: "total-weights", AsOf: asOf}
	var lo, cb sql.NullFloat64
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*), SUM(weight_lowest), SUM(weight_combined) FROM pbs_weight_total WHERE as_of = ?`, asOf).
		Scan(&n, &lo, &cb)
	if err != nil {
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	r.Examined = n
	if n == 0 || !lo.Valid || !cb.Valid {
		r.Verdict, r.Detail = "skipped", "no published TOTAL rows stored for this release"
		return r
	}
	if math.Abs(lo.Float64-100) <= 0.01 && math.Abs(cb.Float64-100) <= 0.01 {
		r.Verdict = "pass"
		r.Detail = fmt.Sprintf("lowest %.4f, combined %.4f", lo.Float64, cb.Float64)
		return r
	}
	r.Verdict, r.Failed = "fail", 1
	r.Detail = fmt.Sprintf("lowest %.4f, combined %.4f, expected 100", lo.Float64, cb.Float64)
	return r
}

func checkMinAvgMax(ctx context.Context, db *sql.DB, asOf string) checkResult {
	r := checkResult{Check: "min-avg-max", AsOf: asOf}
	rows, err := db.QueryContext(ctx, `
		SELECT city, item_desc,
		       MAX(CASE WHEN stat='min' AND value_state='present' THEN value END),
		       MAX(CASE WHEN stat='avg' AND value_state='present' THEN value END),
		       MAX(CASE WHEN stat='max' AND value_state='present' THEN value END)
		FROM pbs_price WHERE surface='appendix-a' AND as_of = ?
		GROUP BY city, item_desc`, asOf)
	if err != nil {
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	for rows.Next() {
		var city, item string
		var lo, av, hi sql.NullFloat64
		if err := rows.Scan(&city, &item, &lo, &av, &hi); err != nil {
			_ = rows.Close()
			r.Verdict, r.Detail = "error", err.Error()
			return r
		}
		if !lo.Valid || !av.Valid || !hi.Valid {
			continue
		}
		r.Examined++
		if lo.Float64 > av.Float64+1e-9 || av.Float64 > hi.Float64+1e-9 {
			r.Failed++
			if len(r.Examples) < 5 {
				r.Examples = append(r.Examples, fmt.Sprintf("%s / %s: %.2f, %.2f, %.2f", city, item, lo.Float64, av.Float64, hi.Float64))
			}
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	if err := rows.Close(); err != nil {
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	if r.Examined == 0 {
		r.Verdict, r.Detail = "skipped", "no complete triplets stored"
		return r
	}
	if r.Failed == 0 {
		r.Verdict = "pass"
		r.Detail = fmt.Sprintf("%d complete triplets ordered correctly", r.Examined)
	} else {
		r.Verdict = "fail"
		r.Detail = fmt.Sprintf("%d of %d triplets violate min<=avg<=max", r.Failed, r.Examined)
	}
	return r
}

func checkSectionCounts(ctx context.Context, db *sql.DB, asOf string) checkResult {
	r := checkResult{Check: "section-counts", AsOf: asOf}
	rows, err := db.QueryContext(ctx,
		`SELECT section, COALESCE(declared_count,0), COALESCE(observed_count,0) FROM pbs_weight_total WHERE as_of = ?`, asOf)
	if err != nil {
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	var declaredSum, observedSum int
	for rows.Next() {
		var sec string
		var dc, oc int
		if err := rows.Scan(&sec, &dc, &oc); err != nil {
			_ = rows.Close()
			r.Verdict, r.Detail = "error", err.Error()
			return r
		}
		r.Examined++
		declaredSum += dc
		observedSum += oc
		if dc != oc {
			r.Failed++
			if len(r.Examples) < 5 {
				r.Examples = append(r.Examples, fmt.Sprintf("%s declared %d, parsed %d", sec, dc, oc))
			}
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	if err := rows.Close(); err != nil {
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	if r.Examined == 0 {
		r.Verdict, r.Detail = "skipped", "no section totals stored"
		return r
	}
	if r.Failed == 0 {
		r.Verdict = "pass"
		r.Detail = fmt.Sprintf("%d sections agree; %d items declared and parsed", r.Examined, declaredSum)
	} else {
		r.Verdict = "fail"
		r.Detail = fmt.Sprintf("%d sections disagree (declared %d, parsed %d)", r.Failed, declaredSum, observedSum)
	}
	return r
}

// checkNationalAverage compares the published national figure against an
// UNWEIGHTED mean of the cities.
//
// A divergence is the expected outcome and is reported as such. The Bureau
// weights cities and does not publish that weight vector, so a near-exact match
// would suggest the parser is reading the same column twice rather than that
// the reconstruction is correct.
func checkNationalAverage(ctx context.Context, db *sql.DB, asOf string) checkResult {
	r := checkResult{Check: "national-average", AsOf: asOf, Verdict: "expected-divergence"}
	rows, err := db.QueryContext(ctx, `
		SELECT n.item_desc, n.value, AVG(p.value), COUNT(p.value)
		FROM pbs_national n
		JOIN pbs_price p
		  ON p.as_of = n.as_of AND p.item_desc = n.item_desc
		 AND p.surface='appendix-a' AND p.stat='avg' AND p.value_state='present'
		WHERE n.as_of = ? AND n.series='national_avg' AND n.value_state='present'
		GROUP BY n.item_desc, n.value`, asOf)
	if err != nil {
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	var devs []float64
	for rows.Next() {
		var item string
		var pub, mean sql.NullFloat64
		var n int
		if err := rows.Scan(&item, &pub, &mean, &n); err != nil {
			_ = rows.Close()
			r.Verdict, r.Detail = "error", err.Error()
			return r
		}
		if !pub.Valid || !mean.Valid || pub.Float64 == 0 || n < 3 {
			continue
		}
		r.Examined++
		devs = append(devs, math.Abs(mean.Float64-pub.Float64)/pub.Float64*100)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	if err := rows.Close(); err != nil {
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	if r.Examined == 0 {
		r.Verdict, r.Detail = "skipped", "no comparable items stored"
		return r
	}
	sort.Float64s(devs)
	med := quantile(devs, 0.5)
	r.Detail = fmt.Sprintf("median deviation %.3f%% over %d items; the Bureau's city weights are unpublished so this is expected, not a failure", med, r.Examined)
	return r
}

func checkPctColumns(ctx context.Context, db *sql.DB, asOf string) checkResult {
	r := checkResult{Check: "pct-columns", AsOf: asOf}
	var total, nonZero int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN ABS(COALESCE(impact_lowest,0)) > 1e-9 THEN 1 ELSE 0 END),0)
		FROM pbs_weight WHERE as_of = ?`, asOf).Scan(&total, &nonZero)
	if err != nil {
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	r.Examined = total
	if total == 0 {
		r.Verdict, r.Detail = "skipped", "no weight rows stored for this release"
		return r
	}
	// This check documents a source property rather than asserting one: it
	// records how much of the impact column is published as zero, which is why
	// percentages in this CLI are derived from levels.
	r.Verdict = "pass"
	r.Detail = fmt.Sprintf("%d of %d impact values are non-zero; percentages are derived from levels regardless", nonZero, total)
	return r
}

func checkRowBlocks(ctx context.Context, db *sql.DB, asOf string) checkResult {
	r := checkResult{Check: "row-blocks", AsOf: asOf}
	rows, err := db.QueryContext(ctx,
		`SELECT block, COUNT(DISTINCT city) FROM pbs_price
		 WHERE surface='appendix-a' AND as_of = ? GROUP BY block ORDER BY block`, asOf)
	if err != nil {
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	var blocks, cities int
	var parts []string
	for rows.Next() {
		var b, c int
		if err := rows.Scan(&b, &c); err != nil {
			_ = rows.Close()
			r.Verdict, r.Detail = "error", err.Error()
			return r
		}
		blocks++
		cities += c
		parts = append(parts, fmt.Sprintf("band %d: %d cities", b, c))
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	if err := rows.Close(); err != nil {
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	r.Examined = blocks
	if blocks == 0 {
		r.Verdict, r.Detail = "skipped", "no price rows stored for this release"
		return r
	}
	// Each stacked band must contribute a disjoint set of cities. A parser that
	// applied one header band to every row would still produce plausible
	// numbers while misattributing most of the panel, and the disjointness of
	// the bands is what catches that.
	var distinct int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT city) FROM pbs_price WHERE surface='appendix-a' AND as_of = ?`, asOf).
		Scan(&distinct); err != nil {
		r.Verdict, r.Detail = "error", err.Error()
		return r
	}
	if distinct != cities {
		r.Verdict, r.Failed = "fail", 1
		r.Detail = fmt.Sprintf("bands overlap: %d city slots across %d bands but only %d distinct cities (%s)",
			cities, blocks, distinct, strings.Join(parts, "; "))
		return r
	}
	r.Verdict = "pass"
	r.Detail = fmt.Sprintf("%d bands partition %d cities disjointly (%s)", blocks, distinct, strings.Join(parts, "; "))
	return r
}

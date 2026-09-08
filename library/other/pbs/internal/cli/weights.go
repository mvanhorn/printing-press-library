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

type weightRow struct {
	Item     string   `json:"item"`
	Unit     string   `json:"unit,omitempty"`
	Section  string   `json:"section,omitempty"`
	Lowest   *float64 `json:"weight_lowest"`
	Combined *float64 `json:"weight_combined"`
}

type weightDiffRow struct {
	Item          string   `json:"item"`
	LowestFrom    *float64 `json:"weight_lowest_from"`
	LowestTo      *float64 `json:"weight_lowest_to"`
	LowestDelta   *float64 `json:"weight_lowest_delta"`
	CombinedFrom  *float64 `json:"weight_combined_from"`
	CombinedTo    *float64 `json:"weight_combined_to"`
	CombinedDelta *float64 `json:"weight_combined_delta"`
	Status        string   `json:"status"`
}

type weightTotalRow struct {
	Section  string   `json:"section"`
	Declared int      `json:"declared_count"`
	Observed int      `json:"observed_count"`
	Lowest   *float64 `json:"weight_lowest"`
	Combined *float64 `json:"weight_combined"`
}

type weightsEnvelope struct {
	AsOf        string           `json:"as_of,omitempty"`
	Weights     []weightRow      `json:"weights,omitempty"`
	Totals      []weightTotalRow `json:"totals,omitempty"`
	TotalLowest *float64         `json:"total_weight_lowest,omitempty"`
	TotalCombo  *float64         `json:"total_weight_combined,omitempty"`
	CheckPassed *bool            `json:"totals_sum_to_100,omitempty"`
	Diff        []weightDiffRow  `json:"diff,omitempty"`
	DiffFrom    string           `json:"diff_from,omitempty"`
	DiffTo      string           `json:"diff_to,omitempty"`
	Note        string           `json:"note,omitempty"`
}

func newNovelWeightsCmd(flags *rootFlags) *cobra.Command {
	var (
		asOf        string
		diff        string
		checkTotal  bool
		dbPath      string
		changedOnly bool
	)
	cmd := &cobra.Command{
		Use:   "weights",
		Short: "Read or diff the item weight vector, and check its totals",
		Long: strings.Trim(`
Inspect the expenditure weight vector the Bureau republishes in every release.

Use this command to read or diff the weight vector and check its totals. Do NOT
use it to compute a price index from those weights; use 'basket' instead.

Only an accruing local store can answer the interesting question here. The
Bureau republishes the whole vector every week and publishes no change log, so
a re-weighting is invisible to anyone who did not keep the previous edition.

--check-total asserts the source's own invariant: the three section TOTAL rows
decompose household expenditure, so they must sum to 100 on both the
lowest-quintile and the combined column. A deviation beyond rounding is a parse
failure, not a finding about the economy.
`, "\n"),
		Example: strings.Trim(`
  pbs-pp-cli weights --check-total
  pbs-pp-cli weights --as-of 2026-09-03 --agent
  pbs-pp-cli weights --diff 2026-08-20..2026-09-03 --changed-only --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "weights")
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
					return printJSONFiltered(cmd.OutOrStdout(), weightsEnvelope{Note: "no local panel yet"}, flags)
				}
				return nil
			}
			defer db.Close()
			warnIfStale(ctx, db.DB(), cmd.ErrOrStderr(), flags, cmd)
			if panelEmpty(ctx, db.DB()) {
				emptyPanelHint(cmd.ErrOrStderr())
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), weightsEnvelope{Note: "the local panel is empty"}, flags)
				}
				return nil
			}
			out := weightsEnvelope{}

			if diff != "" {
				parts := strings.Split(diff, "..")
				if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--diff takes two dates as FROM..TO, e.g. 2026-08-20..2026-09-03"))
				}
				a, b := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
				out.DiffFrom, out.DiffTo = a, b
				wa, err := loadWeights(ctx, db.DB(), a)
				if err != nil {
					return err
				}
				wb, err := loadWeights(ctx, db.DB(), b)
				if err != nil {
					return err
				}
				if len(wa) == 0 || len(wb) == 0 {
					missing := a
					if len(wa) > 0 {
						missing = b
					}
					out.Note = fmt.Sprintf("no stored weight vector for %s; sync that release first", missing)
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
							return err
						}
						return notFoundErr(fmt.Errorf("no stored weight vector for %s", missing))
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n", out.Note)
					return notFoundErr(fmt.Errorf("no stored weight vector for %s", missing))
				}
				seen := map[string]bool{}
				var keys []string
				for k := range wa {
					if !seen[k] {
						seen[k] = true
						keys = append(keys, k)
					}
				}
				for k := range wb {
					if !seen[k] {
						seen[k] = true
						keys = append(keys, k)
					}
				}
				sort.Strings(keys)
				for _, k := range keys {
					ra, okA := wa[k]
					rb, okB := wb[k]
					row := weightDiffRow{Item: k}
					switch {
					case okA && !okB:
						row.Status = "removed_from_basket"
						row.LowestFrom, row.CombinedFrom = ra.Lowest, ra.Combined
					case !okA && okB:
						row.Status = "added_to_basket"
						row.LowestTo, row.CombinedTo = rb.Lowest, rb.Combined
					default:
						row.LowestFrom, row.CombinedFrom = ra.Lowest, ra.Combined
						row.LowestTo, row.CombinedTo = rb.Lowest, rb.Combined
						row.LowestDelta = delta(ra.Lowest, rb.Lowest)
						row.CombinedDelta = delta(ra.Combined, rb.Combined)
						if isZeroish(row.LowestDelta) && isZeroish(row.CombinedDelta) {
							row.Status = "unchanged"
						} else {
							row.Status = "reweighted"
						}
					}
					if changedOnly && row.Status == "unchanged" {
						continue
					}
					out.Diff = append(out.Diff, row)
				}
				if out.Diff == nil {
					out.Diff = []weightDiffRow{}
					out.Note = "no weight changes between those releases"
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "weight vector %s -> %s\n\n", a, b)
				if len(out.Diff) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
					return nil
				}
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, "ITEM\tLOWEST FROM\tLOWEST TO\tDELTA\tSTATUS")
				for _, r := range out.Diff {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", truncate(r.Item, 44),
						fmtPtr(r.LowestFrom), fmtPtr(r.LowestTo), fmtPtr(r.LowestDelta), r.Status)
				}
				return tw.Flush()
			}

			if asOf == "" {
				var d sql.NullString
				if err := db.DB().QueryRowContext(ctx, `SELECT MAX(as_of) FROM pbs_weight`).Scan(&d); err != nil {
					return fmt.Errorf("find latest weight vector: %w", err)
				}
				if d.Valid {
					asOf = d.String
				}
			}
			out.AsOf = asOf
			if asOf == "" {
				out.Note = "no stored weight vectors yet"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
						return err
					}
					return notFoundErr(fmt.Errorf("no stored weight vectors"))
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No stored weight vectors yet.")
				return notFoundErr(fmt.Errorf("no stored weight vectors"))
			}

			w, err := loadWeights(ctx, db.DB(), asOf)
			if err != nil {
				return err
			}
			var keys []string
			for k := range w {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			out.Weights = make([]weightRow, 0, len(keys))
			for _, k := range keys {
				out.Weights = append(out.Weights, w[k])
			}

			tRows, err := db.DB().QueryContext(ctx,
				`SELECT section, COALESCE(declared_count,0), COALESCE(observed_count,0), weight_lowest, weight_combined
				 FROM pbs_weight_total WHERE as_of = ? ORDER BY section`, asOf)
			if err != nil {
				return fmt.Errorf("query weight totals: %w", err)
			}
			var sumLow, sumComb float64
			var haveTotals bool
			for tRows.Next() {
				var sec string
				var dc, oc int
				var lo, cb sql.NullFloat64
				if err := tRows.Scan(&sec, &dc, &oc, &lo, &cb); err != nil {
					_ = tRows.Close()
					return err
				}
				row := weightTotalRow{Section: sec, Declared: dc, Observed: oc}
				if lo.Valid {
					v := lo.Float64
					row.Lowest = &v
					sumLow += v
					haveTotals = true
				}
				if cb.Valid {
					v := cb.Float64
					row.Combined = &v
					sumComb += v
				}
				out.Totals = append(out.Totals, row)
			}
			if err := tRows.Err(); err != nil {
				_ = tRows.Close()
				return err
			}
			if err := tRows.Close(); err != nil {
				return err
			}
			if haveTotals {
				out.TotalLowest, out.TotalCombo = &sumLow, &sumComb
			}

			if checkTotal {
				if !haveTotals {
					out.Note = "no published TOTAL rows stored for this release, so the invariant cannot be checked"
				} else {
					// Whole-percent rendering means the residual is bounded by
					// rounding, not by a fixed epsilon; 0.01 is generous for
					// four-decimal published totals.
					pass := math.Abs(sumLow-100) <= 0.01 && math.Abs(sumComb-100) <= 0.01
					out.CheckPassed = &pass
					if !pass {
						out.Note = fmt.Sprintf("INVARIANT FAILED: section totals sum to %.4f (lowest) and %.4f (combined), not 100. Treat this as a parse failure, not a finding about the data.", sumLow, sumComb)
					}
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
					return err
				}
				if out.CheckPassed != nil && !*out.CheckPassed {
					return apiErr(fmt.Errorf("weight totals do not sum to 100"))
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "weight vector as of %s  (%d items)\n\n", asOf, len(out.Weights))
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ITEM\tUNIT\tLOWEST %\tCOMBINED %\tSECTION")
			for _, r := range out.Weights {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", truncate(r.Item, 44), r.Unit, fmtPtr(r.Lowest), fmtPtr(r.Combined), r.Section)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if haveTotals {
				fmt.Fprintf(cmd.OutOrStdout(), "\npublished section totals sum to %.4f (lowest) and %.4f (combined)\n", sumLow, sumComb)
				for _, t := range out.Totals {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-11s declared %2d items, parsed %2d\n", t.Section, t.Declared, t.Observed)
				}
			}
			if out.CheckPassed != nil {
				if *out.CheckPassed {
					fmt.Fprintln(cmd.OutOrStdout(), "invariant PASSED: totals sum to 100 on both columns")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "invariant FAILED: %s\n", out.Note)
					return apiErr(fmt.Errorf("weight totals do not sum to 100"))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&asOf, "as-of", "", "Release date to read, YYYY-MM-DD (defaults to the newest stored)")
	cmd.Flags().StringVar(&diff, "diff", "", "Compare two releases' weight vectors, as FROM..TO")
	cmd.Flags().BoolVar(&checkTotal, "check-total", false, "Assert the published section totals sum to 100 on both columns")
	cmd.Flags().BoolVar(&changedOnly, "changed-only", false, "With --diff, omit items whose weight did not change")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local panel database path")
	return cmd
}

// loadWeights reads one release's weight vector.
func loadWeights(ctx context.Context, db *sql.DB, asOf string) (map[string]weightRow, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT item_desc, COALESCE(unit,''), COALESCE(section,''), weight_lowest, weight_combined
		 FROM pbs_weight WHERE as_of = ?`, asOf)
	if err != nil {
		return nil, fmt.Errorf("query weights: %w", err)
	}
	out := map[string]weightRow{}
	for rows.Next() {
		var item, unit, sec string
		var lo, cb sql.NullFloat64
		if err := rows.Scan(&item, &unit, &sec, &lo, &cb); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan weight: %w", err)
		}
		r := weightRow{Item: item, Unit: unit, Section: sec}
		if lo.Valid {
			v := lo.Float64
			r.Lowest = &v
		}
		if cb.Valid {
			v := cb.Float64
			r.Combined = &v
		}
		out[item] = r
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return out, nil
}

func delta(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	d := *b - *a
	return &d
}

func isZeroish(p *float64) bool { return p == nil || math.Abs(*p) < 1e-9 }

func fmtPtr(p *float64) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%.4f", *p)
}

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

type moverRow struct {
	Item      string  `json:"item"`
	Unit      string  `json:"unit,omitempty"`
	Current   float64 `json:"current"`
	Compare   float64 `json:"compare_to"`
	PctChange float64 `json:"pct_change"`
	Outlier   bool    `json:"tukey_outlier,omitempty"`
}

type moversEnvelope struct {
	AsOf        string     `json:"as_of"`
	ComparedTo  string     `json:"compared_to"`
	Scope       string     `json:"scope"`
	Window      string     `json:"window"`
	Movers      []moverRow `json:"movers"`
	ItemsRanked int        `json:"items_ranked"`
	Census      cellCensus `json:"census"`
	Note        string     `json:"note,omitempty"`
}

func newNovelMoversCmd(flags *rootFlags) *cobra.Command {
	var (
		asOf     string
		city     string
		national bool
		window   string
		top      int
		flagOut  bool
		dbPath   string
	)
	cmd := &cobra.Command{
		Use:   "movers",
		Short: "Rank every item in a release by percent change recomputed from levels",
		Long: strings.Trim(`
Rank the items in one release by how much their price moved.

Use this command to rank MANY items within ONE release, nationally or for a
named city. Do NOT use it to follow a single item over time; use 'drift'
instead. Do NOT use it to compare one item across cities; use 'spread' instead.

The ranking is recomputed from stored price LEVELS across adjacent releases,
because the Bureau's own percent and impact columns are published as zero on
this data while the level columns are correct. The ranking therefore does not
exist in the source file and cannot be read out of it.

Outliers are flagged with Tukey fences and never dropped.
`, "\n"),
		Example: strings.Trim(`
  pbs-pp-cli movers --top 10
  pbs-pp-cli movers --as-of 2026-09-03 --top 10 --flag-outliers --agent
  pbs-pp-cli movers --city karachi --window cor-wk --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "movers")
			}
			switch strings.ToLower(window) {
			case "", "wow", "prev", "prev-week":
				window = "wow"
			case "cor-wk", "yoy", "corresponding":
				window = "cor-wk"
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--window must be wow or cor-wk, got %q", window))
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
					return printJSONFiltered(cmd.OutOrStdout(), moversEnvelope{
						Movers: []moverRow{}, Note: "no local panel yet"}, flags)
				}
				return nil
			}
			defer db.Close()
			warnIfStale(ctx, db.DB(), cmd.ErrOrStderr(), flags, cmd)
			if panelEmpty(ctx, db.DB()) {
				emptyPanelHint(cmd.ErrOrStderr())
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), moversEnvelope{Movers: []moverRow{}, Note: "the local panel is empty"}, flags)
				}
				return nil
			}

			scope := "national"
			if city != "" {
				resolved, err := resolveCity(ctx, db.DB(), city)
				if err != nil {
					return usageErr(err)
				}
				city = resolved
				scope = city
				national = false
			} else {
				national = true
			}

			if asOf == "" {
				if asOf, err = latestAsOf(ctx, db.DB()); err != nil {
					return err
				}
			}
			out := moversEnvelope{AsOf: asOf, Scope: scope, Window: window, Movers: []moverRow{}}
			if asOf == "" {
				out.Note = "no stored releases yet"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No stored releases yet.")
				return notFoundErr(fmt.Errorf("no stored releases"))
			}

			// Find the comparison release. For week-over-week that is the
			// previous STORED release, which is not necessarily seven days
			// earlier: the upstream index is missing seventeen weeks, so the
			// actual interval is reported rather than assumed.
			var cmpDate string
			if window == "wow" {
				var d sql.NullString
				if err := db.DB().QueryRowContext(ctx,
					`SELECT MAX(as_of) FROM pbs_price WHERE surface='appendix-a' AND as_of < ?`, asOf).Scan(&d); err != nil {
					return fmt.Errorf("find previous release: %w", err)
				}
				if d.Valid {
					cmpDate = d.String
				}
			} else {
				var d sql.NullString
				if err := db.DB().QueryRowContext(ctx,
					`SELECT MAX(as_of) FROM pbs_price WHERE surface='appendix-a' AND as_of <= date(?, '-1 year')`, asOf).Scan(&d); err != nil {
					return fmt.Errorf("find corresponding release: %w", err)
				}
				if d.Valid {
					cmpDate = d.String
				}
			}
			out.ComparedTo = cmpDate
			if cmpDate == "" {
				out.Note = fmt.Sprintf("no stored release to compare %s against for window %s; sync a wider range first", asOf, window)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", out.Note)
				return notFoundErr(fmt.Errorf("no comparison release for %s", asOf))
			}

			cur, cen1, err := itemLevels(ctx, db.DB(), asOf, city, national)
			if err != nil {
				return err
			}
			prev, cen2, err := itemLevels(ctx, db.DB(), cmpDate, city, national)
			if err != nil {
				return err
			}
			out.Census = cellCensus{
				Present: cen1.Present + cen2.Present, Zero: cen1.Zero + cen2.Zero,
				Blank: cen1.Blank + cen2.Blank, NA: cen1.NA + cen2.NA,
				Unparseable: cen1.Unparseable + cen2.Unparseable,
			}

			var rows []moverRow
			for item, c := range cur {
				p, ok := prev[item]
				if !ok || p.val == 0 {
					continue
				}
				rows = append(rows, moverRow{
					Item: item, Unit: c.unit, Current: c.val, Compare: p.val,
					PctChange: (c.val - p.val) / p.val * 100,
				})
			}
			out.ItemsRanked = len(rows)
			if flagOut {
				// Tukey fences are computed over the items that ACTUALLY MOVED,
				// not the whole basket.
				//
				// This release's change distribution is zero-inflated: a
				// majority of the basket is unchanged in a typical week
				// (measured at 27 of 51 items), which drags Q1 to exactly zero
				// and shrinks the interquartile range until almost every real
				// move sits outside the fences. Fencing the full basket
				// therefore flagged every mover, which is not information.
				// Asking "among the items that moved, which moved unusually?"
				// is the question a caller actually has.
				moved := make([]float64, 0, len(rows))
				for _, r := range rows {
					if math.Abs(r.PctChange) > 1e-9 {
						moved = append(moved, r.PctChange)
					}
				}
				if len(moved) < 4 {
					out.Note = strings.TrimSpace(fmt.Sprintf(
						"%s outlier flagging withheld: only %d of %d items moved, too few to fence.",
						out.Note, len(moved), len(rows)))
				} else {
					sort.Float64s(moved)
					q1, q3 := quantile(moved, 0.25), quantile(moved, 0.75)
					iqr := q3 - q1
					if iqr <= 1e-9 {
						out.Note = strings.TrimSpace(fmt.Sprintf(
							"%s outlier flagging withheld: the %d moving items share one change value, so there is no spread to fence.",
							out.Note, len(moved)))
					} else {
						lo, hi := q1-1.5*iqr, q3+1.5*iqr
						flagged := 0
						for i := range rows {
							if math.Abs(rows[i].PctChange) <= 1e-9 {
								continue
							}
							if rows[i].PctChange < lo || rows[i].PctChange > hi {
								rows[i].Outlier = true
								flagged++
							}
						}
						out.Note = strings.TrimSpace(fmt.Sprintf(
							"%s outliers fenced over the %d items that moved (%d of %d items were unchanged); %d flagged.",
							out.Note, len(moved), len(rows)-len(moved), len(rows), flagged))
					}
				}
			}
			sort.SliceStable(rows, func(i, j int) bool {
				return math.Abs(rows[i].PctChange) > math.Abs(rows[j].PctChange)
			})
			if top > 0 && len(rows) > top {
				rows = rows[:top]
			}
			out.Movers = rows

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No items comparable between %s and %s.\n", asOf, cmpDate)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s vs %s  (%s, %s)\n\n", asOf, cmpDate, scope, window)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ITEM\tUNIT\tNOW\tTHEN\t% CHANGE\tFLAG")
			for _, r := range rows {
				flag := ""
				if r.Outlier {
					flag = "tukey-outlier"
				}
				fmt.Fprintf(tw, "%s\t%s\t%.2f\t%.2f\t%+.2f\t%s\n", truncate(r.Item, 46), r.Unit, r.Current, r.Compare, r.PctChange, flag)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d items ranked from stored levels; the source's own percent columns are published as zero on this data.\n", out.ItemsRanked)
			if ex := out.Census.Excluded(); ex > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%d cells excluded across both releases: %d uncollected zeros, %d blank, %d not-available.\n",
					ex, out.Census.Zero, out.Census.Blank, out.Census.NA)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&asOf, "as-of", "", "Release date to rank, YYYY-MM-DD (defaults to the newest stored)")
	cmd.Flags().StringVar(&city, "city", "", "Rank prices for one city instead of nationally")
	cmd.Flags().BoolVar(&national, "national", false, "Rank the published national averages (the default)")
	cmd.Flags().StringVar(&window, "window", "wow", "Comparison window: wow (previous stored release) or cor-wk (about one year earlier)")
	cmd.Flags().IntVar(&top, "top", 0, "Show only the N largest absolute moves (0 shows all)")
	cmd.Flags().BoolVar(&flagOut, "flag-outliers", false, "Flag moves outside Tukey fences (they are never dropped)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local panel database path")
	return cmd
}

type levelCell struct {
	val  float64
	unit string
}

// itemLevels loads one release's price level per item, either the published
// national average or one city's average.
func itemLevels(ctx context.Context, db *sql.DB, asOf, city string, national bool) (map[string]levelCell, cellCensus, error) {
	out := map[string]levelCell{}
	var cen cellCensus
	if national {
		rows, err := db.QueryContext(ctx,
			`SELECT item_desc, value, value_state FROM pbs_national
			 WHERE as_of = ? AND series = 'national_avg'`, asOf)
		if err != nil {
			return nil, cen, fmt.Errorf("query national levels: %w", err)
		}
		for rows.Next() {
			var item, state string
			var v sql.NullFloat64
			if err := rows.Scan(&item, &v, &state); err != nil {
				_ = rows.Close()
				return nil, cen, err
			}
			if state == "present" && v.Valid {
				out[item] = levelCell{val: v.Float64}
				cen.Present++
			} else {
				countState(&cen, state)
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, cen, err
		}
		if err := rows.Close(); err != nil {
			return nil, cen, err
		}
		return out, cen, nil
	}
	rows, err := db.QueryContext(ctx,
		`SELECT item_desc, COALESCE(unit,''), value, value_state FROM pbs_price
		 WHERE surface='appendix-a' AND as_of = ? AND city = ? AND stat = 'avg'`, asOf, city)
	if err != nil {
		return nil, cen, fmt.Errorf("query city levels: %w", err)
	}
	for rows.Next() {
		var item, unit, state string
		var v sql.NullFloat64
		if err := rows.Scan(&item, &unit, &v, &state); err != nil {
			_ = rows.Close()
			return nil, cen, err
		}
		if state == "present" && v.Valid {
			out[item] = levelCell{val: v.Float64, unit: unit}
			cen.Present++
		} else {
			countState(&cen, state)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, cen, err
	}
	if err := rows.Close(); err != nil {
		return nil, cen, err
	}
	return out, cen, nil
}

func countState(c *cellCensus, state string) {
	switch state {
	case "zero":
		c.Zero++
	case "blank":
		c.Blank++
	case "na":
		c.NA++
	default:
		c.Unparseable++
	}
}

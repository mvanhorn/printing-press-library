// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local
// Reads only the accrued local panel; no upstream call. Sync advances the data.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type basketComponent struct {
	Item      string  `json:"item"`
	Weight    float64 `json:"weight"`
	Price     float64 `json:"price"`
	BasePrice float64 `json:"base_price,omitempty"`
	Relative  float64 `json:"relative,omitempty"`
}

type basketPoint struct {
	AsOf string `json:"as_of"`
	// Value is a weighted mean PRICE IN RUPEES when no --rebase is given, and a
	// rebased index number when one is. Measure says which, because the two are
	// not interchangeable and a caller must not read a mean price as an index.
	Value      *float64          `json:"value"`
	Measure    string            `json:"measure"`
	WeightUsed float64           `json:"weight_share_used_pct"`
	Items      int               `json:"items_used"`
	Skipped    []string          `json:"items_skipped,omitempty"`
	Components []basketComponent `json:"components,omitempty"`
}

type basketEnvelope struct {
	Scope      string        `json:"scope"`
	WeightCol  string        `json:"weight_column"`
	Rebase     string        `json:"rebase,omitempty"`
	Points     []basketPoint `json:"points"`
	Unweighted bool          `json:"cross_city_unweighted"`
	Note       string        `json:"note,omitempty"`
}

func newNovelBasketCmd(flags *rootFlags) *cobra.Command {
	var (
		items     []string
		cityFlag  string
		weightCol string
		asOf      string
		from      string
		to        string
		rebase    string
		dbPath    string
		showParts bool
	)
	cmd := &cobra.Command{
		Use:   "basket",
		Short: "Build a price index over any item subset using the published weights",
		Long: strings.Trim(`
Compute your own weighted price index over a chosen subset of items.

Use this command to compute an index over a chosen item subset. Do NOT use it
to inspect or diff the weight vector itself; use 'weights' instead.

Weights come from the vector the Bureau republishes in every release, so the
index is built from the source's own expenditure shares rather than invented
ones. Because the chosen subset is only part of the basket, the weights are
renormalised over the items actually used and the weight share consumed is
always reported.

IMPORTANT LIMIT, stated rather than hidden: the Bureau weights CITIES when it
computes a national figure, and it does not publish that city weight vector.
So a per-city index here is properly weighted across items, while any
cross-city figure is an explicitly UNWEIGHTED mean of cities and is labelled as
such. This command does not attempt to reproduce the published national
average; see 'verify' for why that reconciliation is expected to diverge.

Without --rebase the result is a weighted mean PRICE IN RUPEES, not an index.
Pass --rebase to express it as an index against a chosen release = 100. The
measure field always says which of the two you were given.
`, "\n"),
		Example: strings.Trim(`
  pbs-pp-cli basket --item "Wheat Flour Bag" --item "Sugar Refined" --weight lowest
  pbs-pp-cli basket --item Onions --item Potatoes --item Tomatoes --city lahore --json
  pbs-pp-cli basket --item "Petrol Super" --item "Hi-Speed Diesel" --from 2026-08-01 --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "basket")
			}
			switch strings.ToLower(weightCol) {
			case "", "lowest", "q1":
				weightCol = "weight_lowest"
			case "combined", "all":
				weightCol = "weight_combined"
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--weight must be lowest or combined, got %q", weightCol))
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
					return printJSONFiltered(cmd.OutOrStdout(), basketEnvelope{
						Points: []basketPoint{}, Note: "no local panel yet"}, flags)
				}
				return nil
			}
			defer db.Close()
			warnIfStale(ctx, db.DB(), cmd.ErrOrStderr(), flags, cmd)
			if panelEmpty(ctx, db.DB()) {
				emptyPanelHint(cmd.ErrOrStderr())
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), basketEnvelope{Points: []basketPoint{}, Note: "the local panel is empty"}, flags)
				}
				return nil
			}

			var want []string
			for _, it := range items {
				for _, part := range strings.Split(it, ",") {
					part = strings.TrimSpace(part)
					if part == "" {
						continue
					}
					resolved, err := resolveItem(ctx, db.DB(), part)
					if err != nil {
						return usageErr(err)
					}
					want = append(want, resolved)
				}
			}
			if len(want) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one --item is required; an index over the whole basket is the Bureau's own published SPI, which needs no rebuilding"))
			}
			sort.Strings(want)

			scope := "cross-city (unweighted)"
			unweighted := true
			if cityFlag != "" {
				resolved, err := resolveCity(ctx, db.DB(), cityFlag)
				if err != nil {
					return usageErr(err)
				}
				cityFlag = resolved
				scope = cityFlag
				unweighted = false
			}
			out := basketEnvelope{Scope: scope, WeightCol: weightCol, Rebase: rebase,
				Points: []basketPoint{}, Unweighted: unweighted}
			if unweighted {
				out.Note = "cross-city figures are an unweighted mean of cities: the Bureau does not publish its city weight vector, so a weighted national reconstruction is not possible from these files"
			}

			dates, err := basketDates(ctx, db.DB(), asOf, from, to)
			if err != nil {
				return err
			}
			if len(dates) == 0 {
				out.Note = strings.TrimSpace(out.Note + " no stored releases in range")
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
						return err
					}
					return notFoundErr(fmt.Errorf("no stored releases in range"))
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No stored releases in range.")
				return notFoundErr(fmt.Errorf("no stored releases in range"))
			}

			// A rebased index needs a base period's prices.
			base := map[string]float64{}
			if rebase != "" {
				base, err = basketPrices(ctx, db.DB(), rebase, want, cityFlag, unweighted)
				if err != nil {
					return err
				}
				if len(base) == 0 {
					return notFoundErr(fmt.Errorf("no stored prices for the rebase date %s", rebase))
				}
			}

			for _, d := range dates {
				prices, err := basketPrices(ctx, db.DB(), d, want, cityFlag, unweighted)
				if err != nil {
					return err
				}
				weights, err := basketWeights(ctx, db.DB(), d, want, weightCol)
				if err != nil {
					return err
				}
				p := basketPoint{AsOf: d}
				var num, den float64
				for _, item := range want {
					price, hasPrice := prices[item]
					w, hasWeight := weights[item]
					if !hasPrice || !hasWeight || w <= 0 {
						reason := "no price"
						if hasPrice && !hasWeight {
							reason = "no weight"
						} else if hasPrice && w <= 0 {
							reason = "zero weight"
						}
						p.Skipped = append(p.Skipped, fmt.Sprintf("%s (%s)", item, reason))
						continue
					}
					comp := basketComponent{Item: item, Weight: w, Price: price}
					rel := price
					if rebase != "" {
						b, okB := base[item]
						if !okB || b == 0 {
							p.Skipped = append(p.Skipped, fmt.Sprintf("%s (no base price)", item))
							continue
						}
						rel = price / b * 100
						comp.BasePrice, comp.Relative = b, rel
					}
					num += w * rel
					den += w
					p.Items++
					if showParts {
						p.Components = append(p.Components, comp)
					}
				}
				if den > 0 {
					v := num / den
					p.Value = &v
					p.WeightUsed = den
					if rebase != "" {
						p.Measure = "index_" + rebase + "_eq_100"
					} else {
						p.Measure = "weighted_mean_price_pkr"
					}
				}
				out.Points = append(out.Points, p)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "basket of %d items, %s weights, scope %s\n", len(want), weightCol, scope)
			if rebase != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "rebased to %s = 100\n", rebase)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			tw := newTabWriter(cmd.OutOrStdout())
			header := "AS OF\tWEIGHTED MEAN PRICE (PKR)\tITEMS USED\tWEIGHT SHARE %\tSKIPPED"
			if rebase != "" {
				header = "AS OF\tINDEX (" + rebase + "=100)\tITEMS USED\tWEIGHT SHARE %\tSKIPPED"
			}
			fmt.Fprintln(tw, header)
			for _, p := range out.Points {
				val := "-"
				if p.Value != nil {
					val = fmt.Sprintf("%.4f", *p.Value)
				}
				fmt.Fprintf(tw, "%s\t%s\t%d\t%.4f\t%d\n", p.AsOf, val, p.Items, p.WeightUsed, len(p.Skipped))
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if last := out.Points[len(out.Points)-1]; len(last.Skipped) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nskipped in %s: %s\n", last.AsOf, strings.Join(last.Skipped, "; "))
			}
			if out.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", out.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&items, "item", nil, "Item to include; repeatable, or comma-separated (required)")
	cmd.Flags().StringVar(&cityFlag, "city", "", "Compute the index for one city (omit for an unweighted cross-city mean)")
	cmd.Flags().StringVar(&weightCol, "weight", "lowest", "Which published weight column to use: lowest or combined")
	cmd.Flags().StringVar(&asOf, "as-of", "", "Single release date, YYYY-MM-DD (defaults to the newest stored)")
	cmd.Flags().StringVar(&from, "from", "", "Earliest release date for a series, YYYY-MM-DD")
	cmd.Flags().StringVar(&to, "to", "", "Latest release date for a series, YYYY-MM-DD")
	cmd.Flags().StringVar(&rebase, "rebase", "", "Express the index relative to this release = 100, YYYY-MM-DD")
	cmd.Flags().BoolVar(&showParts, "show-components", false, "Include each item's weight and price")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local panel database path")
	return cmd
}

func basketDates(ctx context.Context, db *sql.DB, asOf, from, to string) ([]string, error) {
	if asOf != "" {
		return []string{asOf}, nil
	}
	if from == "" && to == "" {
		var d sql.NullString
		if err := db.QueryRowContext(ctx, `SELECT MAX(as_of) FROM pbs_weight`).Scan(&d); err != nil {
			return nil, fmt.Errorf("find latest release: %w", err)
		}
		if !d.Valid || d.String == "" {
			return nil, nil
		}
		return []string{d.String}, nil
	}
	q := `SELECT DISTINCT as_of FROM pbs_weight WHERE 1=1`
	var args []any
	if from != "" {
		q += ` AND as_of >= ?`
		args = append(args, from)
	}
	if to != "" {
		q += ` AND as_of <= ?`
		args = append(args, to)
	}
	q += ` ORDER BY as_of`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query release dates: %w", err)
	}
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, d)
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

// basketPrices loads the price level per item for one release.
func basketPrices(ctx context.Context, db *sql.DB, asOf string, items []string, city string, unweighted bool) (map[string]float64, error) {
	out := map[string]float64{}
	if city != "" {
		rows, err := db.QueryContext(ctx,
			`SELECT item_desc, value FROM pbs_price
			 WHERE surface='appendix-a' AND as_of = ? AND city = ? AND stat='avg' AND value_state='present'`,
			asOf, city)
		if err != nil {
			return nil, fmt.Errorf("query basket prices: %w", err)
		}
		if err := scanItemFloats(rows, out); err != nil {
			return nil, err
		}
		return filterKeys(out, items), nil
	}
	// Cross-city: use the published national average where available, which is
	// the Bureau's own city-weighted figure, and fall back to nothing rather
	// than to an invented mean.
	rows, err := db.QueryContext(ctx,
		`SELECT item_desc, value FROM pbs_national
		 WHERE as_of = ? AND series='national_avg' AND value_state='present'`, asOf)
	if err != nil {
		return nil, fmt.Errorf("query national basket prices: %w", err)
	}
	if err := scanItemFloats(rows, out); err != nil {
		return nil, err
	}
	return filterKeys(out, items), nil
}

func basketWeights(ctx context.Context, db *sql.DB, asOf string, items []string, col string) (map[string]float64, error) {
	// The column name is chosen from a closed set above, never interpolated
	// from user input.
	q := `SELECT item_desc, weight_lowest FROM pbs_weight WHERE as_of = ?`
	if col == "weight_combined" {
		q = `SELECT item_desc, weight_combined FROM pbs_weight WHERE as_of = ?`
	}
	rows, err := db.QueryContext(ctx, q, asOf)
	if err != nil {
		return nil, fmt.Errorf("query basket weights: %w", err)
	}
	out := map[string]float64{}
	if err := scanItemFloats(rows, out); err != nil {
		return nil, err
	}
	return filterKeys(out, items), nil
}

func scanItemFloats(rows *sql.Rows, into map[string]float64) error {
	for rows.Next() {
		var item string
		var v sql.NullFloat64
		if err := rows.Scan(&item, &v); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan: %w", err)
		}
		if v.Valid {
			into[item] = v.Float64
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	return rows.Close()
}

func filterKeys(m map[string]float64, keep []string) map[string]float64 {
	out := make(map[string]float64, len(keep))
	for _, k := range keep {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out
}

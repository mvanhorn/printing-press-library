// Copyright 2026 Dickie and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored transcendence command. Not generated. Unions live holdings
// across every entity in the local mirror and computes amount-weighted
// aggregates (weighted-average yield and days-to-maturity) plus an optional
// grouped breakdown — the consolidated group-treasurer view the entity-scoped
// API never returns from a single call.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func newNovelBookCmd(flags *rootFlags) *cobra.Command {
	var flagGroupBy string
	var dbPath string
	var includeRedeemed bool

	cmd := &cobra.Command{
		Use:   "book",
		Short: "Consolidated portfolio across every entity: total invested, weighted yield/maturity",
		Long: strings.Trim(`
Aggregates live holdings across every entity in the local mirror into one
portfolio: total invested, amount-weighted average yield (WAY) and days to
maturity (WAM), and a per-currency split. --group-by adds a breakdown by one
or more of: currency, obligor, entity, status, cell, maturity-bucket.

Run `+"`ts-pp-cli sync`"+` first to populate the mirror.`, "\n"),
		Example: strings.Trim(`
  ts-pp-cli book --json
  ts-pp-cli book --group-by currency
  ts-pp-cli book --group-by currency,maturity-bucket`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			validDims := map[string]bool{"currency": true, "obligor": true, "entity": true, "status": true, "cell": true, "maturity-bucket": true}
			var dims []string
			for _, d := range strings.Split(flagGroupBy, ",") {
				d = strings.ToLower(strings.TrimSpace(d))
				if d == "" {
					continue
				}
				if !validDims[d] {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--group-by dimensions must be among: currency, obligor, entity, status, cell, maturity-bucket"))
				}
				dims = append(dims, d)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, ok, err := openMirror(ctx, cmd, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				return emitEmpty(cmd, flags)
			}
			defer db.Close()

			where := activeHoldingWhere
			if includeRedeemed {
				where = "1=1"
			}
			holidays := loadHolidaySet(ctx, db)
			// Weighted-average yield uses the realised/contracted yield, falling
			// back to final_yield. next_yield (the next period's reset for
			// extendables) is forward-looking and is intentionally excluded so it
			// cannot skew the current WAY.
			q := `SELECT COALESCE(value,0), COALESCE(yield, final_yield), maturity_date,
				COALESCE(currency,''), COALESCE(entity_code,''), COALESCE(obligor_exposure_code,''),
				COALESCE(status,''), COALESCE(cell_code,'')
				FROM holding WHERE ` + where

			rows, err := db.DB().QueryContext(ctx, q)
			if err != nil {
				return fmt.Errorf("querying holdings: %w", err)
			}
			defer rows.Close()

			type agg struct {
				value    float64
				yieldNum float64 // sum(value*yield) over rows with a yield
				yieldDen float64 // sum(value) over rows with a yield
				dayNum   float64 // sum(value*days) over rows with a maturity date
				dayDen   float64 // sum(value) over rows with a maturity date
				holdings int
			}
			now := time.Now()
			total := agg{}
			ccySplit := map[string]float64{}
			groups := map[string]*agg{}
			groupOrder := []string{}

			add := func(a *agg, value, yield float64, hasYield bool, days float64, hasDays bool) {
				a.value += value
				a.holdings++
				if hasYield {
					a.yieldNum += value * yield
					a.yieldDen += value
				}
				if hasDays {
					a.dayNum += value * days
					a.dayDen += value
				}
			}

			for rows.Next() {
				var value sql.NullFloat64
				var yield sql.NullFloat64
				var md sql.NullString
				var ccy, ent, obl, status, cell sql.NullString
				if err := rows.Scan(&value, &yield, &md, &ccy, &ent, &obl, &status, &cell); err != nil {
					continue
				}
				v := value.Float64
				var days float64
				hasDays := false
				if md.Valid {
					if t, ok := parseStoredDate(md.String); ok {
						days = t.Sub(now).Hours() / 24
						// Exclude already-matured-but-not-yet-settled active
						// holdings (negative days, e.g. inside a T+2 window or
						// after a stale sync) so they don't drag WAM toward zero.
						if days >= 0 {
							hasDays = true
						}
					}
				}
				add(&total, v, yield.Float64, yield.Valid, days, hasDays)
				ccySplit[ccy.String] += v

				if len(dims) > 0 {
					parts := make([]string, 0, len(dims))
					for _, d := range dims {
						switch d {
						case "currency":
							parts = append(parts, ccy.String)
						case "obligor":
							parts = append(parts, obl.String)
						case "entity":
							parts = append(parts, ent.String)
						case "status":
							parts = append(parts, status.String)
						case "cell":
							parts = append(parts, cell.String)
						case "maturity-bucket":
							b := "unknown"
							if md.Valid {
								if t, ok := parseStoredDate(md.String); ok {
									b = adjustSettlement(t, holidays).Format("2006-01")
								}
							}
							parts = append(parts, b)
						}
					}
					key := strings.Join(parts, " / ")
					g := groups[key]
					if g == nil {
						g = &agg{}
						groups[key] = g
						groupOrder = append(groupOrder, key)
					}
					add(g, v, yield.Float64, yield.Valid, days, hasDays)
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("scanning holdings: %w", err)
			}

			way := func(a agg) float64 {
				if a.yieldDen > 0 {
					return a.yieldNum / a.yieldDen
				}
				return 0
			}
			wam := func(a agg) float64 {
				if a.dayDen > 0 {
					return a.dayNum / a.dayDen
				}
				return 0
			}

			type groupView struct {
				Key      string  `json:"key"`
				Invested float64 `json:"invested"`
				Share    float64 `json:"share"`
				WAYield  float64 `json:"weighted_avg_yield"`
				WAMDays  float64 `json:"weighted_avg_maturity_days"`
				Holdings int     `json:"holdings"`
			}
			groupViews := make([]groupView, 0, len(groupOrder))
			for _, k := range groupOrder {
				g := groups[k]
				share := 0.0
				if total.value > 0 {
					share = g.value / total.value
				}
				groupViews = append(groupViews, groupView{
					Key: k, Invested: g.value, Share: share,
					WAYield: way(*g), WAMDays: wam(*g), Holdings: g.holdings,
				})
			}
			sort.Slice(groupViews, func(i, j int) bool { return groupViews[i].Invested > groupViews[j].Invested })

			ccyOut := make(map[string]float64, len(ccySplit))
			for k, v := range ccySplit {
				ccyOut[k] = v
			}

			view := struct {
				TotalInvested float64            `json:"total_invested"`
				WAYield       float64            `json:"weighted_avg_yield"`
				WAMDays       float64            `json:"weighted_avg_maturity_days"`
				Holdings      int                `json:"holdings"`
				CurrencySplit map[string]float64 `json:"currency_split"`
				Groups        []groupView        `json:"groups,omitempty"`
			}{
				TotalInvested: total.value,
				WAYield:       way(total),
				WAMDays:       wam(total),
				Holdings:      total.holdings,
				CurrencySplit: ccyOut,
				Groups:        groupViews,
			}

			if jsonMode(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			if total.holdings == 0 {
				fmt.Fprintln(out, "no active holdings found")
				return nil
			}
			fmt.Fprintf(out, "Total invested: %.2f across %d holdings\n", view.TotalInvested, view.Holdings)
			fmt.Fprintf(out, "Weighted-avg yield: %.3f%%   Weighted-avg maturity: %.0f days\n", view.WAYield*100, view.WAMDays)
			fmt.Fprintln(out, "Currency split:")
			ccyKeys := make([]string, 0, len(ccyOut))
			for k := range ccyOut {
				ccyKeys = append(ccyKeys, k)
			}
			sort.Strings(ccyKeys)
			for _, k := range ccyKeys {
				fmt.Fprintf(out, "  %-6s %.2f\n", k, ccyOut[k])
			}
			if len(groupViews) > 0 {
				fmt.Fprintln(out, "")
				tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "GROUP\tINVESTED\tSHARE\tWA-YIELD\tWAM-DAYS\tHOLDINGS")
				for _, g := range groupViews {
					fmt.Fprintf(tw, "%s\t%.2f\t%.1f%%\t%.3f%%\t%.0f\t%d\n", g.Key, g.Invested, g.Share*100, g.WAYield*100, g.WAMDays, g.Holdings)
				}
				_ = tw.Flush()
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "", "Comma list: currency, obligor, entity, status, cell, maturity-bucket")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard cache location)")
	cmd.Flags().BoolVar(&includeRedeemed, "include-redeemed", false, "Include redeemed/cancelled holdings")
	return cmd
}

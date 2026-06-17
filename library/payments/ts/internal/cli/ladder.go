// Copyright 2026 Dickie and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored transcendence command. Not generated. Joins holding maturity
// dates to the mirrored holidays calendar and buckets settlement-adjusted cash
// by week or month across all entities — a portfolio-level view no single API
// call returns.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newNovelLadderCmd(flags *rootFlags) *cobra.Command {
	var flagBy string
	var flagCurrency string
	var dbPath string
	var includeRedeemed bool

	cmd := &cobra.Command{
		Use:   "ladder",
		Short: "Cash-flow forecast: settlement-adjusted maturity ladder across all entities",
		Long: strings.Trim(`
Projects cash landing back into the account, bucketed by week or month, with
each holding's maturity date shifted forward past weekends and mirrored
holidays to its real settlement date. Aggregates across every entity in the
local mirror.

Run `+"`ts-pp-cli sync`"+` first to populate the mirror.`, "\n"),
		Example: strings.Trim(`
  ts-pp-cli ladder --by week --currency USD --json
  ts-pp-cli ladder --by month`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			by := strings.ToLower(strings.TrimSpace(flagBy))
			if by == "" {
				by = "week"
			}
			if by != "week" && by != "month" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--by must be 'week' or 'month'"))
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

			holidays := loadHolidaySet(ctx, db)

			where := activeHoldingWhere
			if includeRedeemed {
				where = "1=1"
			}
			q := `SELECT maturity_date, currency, maturity_value FROM holding WHERE maturity_date IS NOT NULL AND ` + where
			var qargs []any
			if c := strings.ToUpper(strings.TrimSpace(flagCurrency)); c != "" {
				q += " AND currency = ?"
				qargs = append(qargs, c)
			}

			rows, err := db.DB().QueryContext(ctx, q, qargs...)
			if err != nil {
				return fmt.Errorf("querying holdings: %w", err)
			}
			defer rows.Close()

			type bucket struct {
				Bucket          string  `json:"bucket"`
				Currency        string  `json:"currency"`
				MaturingValue   float64 `json:"maturing_value"`
				Holdings        int     `json:"holdings"`
				FirstSettlement string  `json:"first_settlement"`
			}
			agg := map[string]*bucket{}
			scanned := 0
			for rows.Next() {
				var md, cur sql.NullString
				var mv sql.NullFloat64
				if err := rows.Scan(&md, &cur, &mv); err != nil {
					continue
				}
				scanned++
				// A holding with no maturity date or no maturity value cannot
				// contribute to a cash-flow forecast; skip it entirely rather
				// than count it with a zero value (which would inflate the
				// per-bucket holding count without adding cash).
				if !md.Valid || !mv.Valid {
					continue
				}
				t, ok := parseStoredDate(md.String)
				if !ok {
					continue
				}
				t = adjustSettlement(t, holidays)
				var key string
				if by == "week" {
					y, w := t.ISOWeek()
					key = fmt.Sprintf("%04d-W%02d", y, w)
				} else {
					key = t.Format("2006-01")
				}
				settle := t.Format("2006-01-02")
				k := key + "|" + cur.String
				b := agg[k]
				if b == nil {
					b = &bucket{Bucket: key, Currency: cur.String, FirstSettlement: settle}
					agg[k] = b
				}
				b.MaturingValue += mv.Float64
				b.Holdings++
				if settle < b.FirstSettlement {
					b.FirstSettlement = settle
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("scanning holdings: %w", err)
			}

			out := make([]bucket, 0, len(agg))
			for _, b := range agg {
				out = append(out, *b)
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Bucket != out[j].Bucket {
					return out[i].Bucket < out[j].Bucket
				}
				return out[i].Currency < out[j].Currency
			})

			if jsonMode(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no maturing holdings found (scanned %d)\n", scanned)
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "BUCKET\tCCY\tMATURING\tHOLDINGS\tFIRST SETTLEMENT")
			for _, b := range out {
				fmt.Fprintf(tw, "%s\t%s\t%.2f\t%d\t%s\n", b.Bucket, b.Currency, b.MaturingValue, b.Holdings, b.FirstSettlement)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagBy, "by", "week", "Bucket maturities by 'week' or 'month'")
	cmd.Flags().StringVar(&flagCurrency, "currency", "", "Filter to a single currency (e.g. USD)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard cache location)")
	cmd.Flags().BoolVar(&includeRedeemed, "include-redeemed", false, "Include redeemed/cancelled holdings")
	return cmd
}

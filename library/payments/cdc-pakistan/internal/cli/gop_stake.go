// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"
)

type gopRow struct {
	ISIN       string  `json:"isin"`
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	Status     string  `json:"status,omitempty"`
	PaidUpIncl float64 `json:"paid_up_incl_gop"`
	PaidUpExcl float64 `json:"paid_up_excl_gop"`
	GopShares  float64 `json:"gop_shares"`
	GopPct     float64 `json:"gop_pct"`
}

type gopView struct {
	Vintage string   `json:"vintage"`
	Rows    []gopRow `json:"rows"`
	Scanned int      `json:"scanned_rows"`
	WithGop int      `json:"rows_with_gop_stake"`
	Note    string   `json:"note"`
	Caveat  string   `json:"caveat"`
}

func newNovelGopStakeCmd(flags *rootFlags) *cobra.Command {
	var (
		vintage string
		minPct  float64
		symbol  string
		limit   int
		dbPath  string
	)

	cmd := &cobra.Command{
		Use:   "stake",
		Short: "Derive state-held capital per security from the difference between the including-GoP and excluding-GoP capital columns.",
		Long: strings.Trim(`
Per-security Government of Pakistan shareholding.

CDC publishes paid-up capital twice per security -- once including GoP holding
and once excluding it -- so the difference is the state-held capital. No other
Pakistani source publishes per-symbol state ownership in machine-readable form,
and the exchange carries heavy state-owned-enterprise weight, so this is a
genuinely unavailable cross-section.

LEVEL ONLY. There is no time series and this command will not pretend
otherwise: exactly ONE live vintage of the source report exists at a time
because CDC removes prior months from its site, and the one archived vintage
available for comparison has an incompatible column set. A delta series was
scoped and deliberately KILLED rather than shipped as an empty promise.

Rows where the two columns are equal have no state holding and are excluded from
the result unless --min-pct is 0.
`, "\n"),
		Example:     "  cdc-pakistan-pp-cli gop stake --min-pct 25 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--min-pct=25;--limit=5", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "gop stake")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("cdc-pakistan-pp-cli")
			}
			view := &gopView{Vintage: vintage, Rows: []gopRow{}, Caveat: gopCaveat}
			db, ready, oerr := openCDCMirror(ctx, dbPath, "cdc_penetration_rows")
			if oerr != nil {
				return oerr
			}
			if !ready {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: cdc-pakistan-pp-cli verify rows --vintage DATE --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}
			defer db.Close()

			// Scope to ONE vintage. Without this the query blends every stored
			// vintage and returns each issuer once per vintage.
			scoped := strings.TrimSpace(vintage)
			if scoped == "" {
				nv, nerr := newestVintage(ctx, db)
				if nerr != nil {
					return apiErr(nerr)
				}
				scoped = nv
			}
			view.Vintage = scoped

			// Only part A carries paid-up capital; part B is funds, which have
			// units and no share capital at all.
			q := `SELECT isin, COALESCE(symbol,''), COALESCE(name,''), COALESCE(status,''),
			             paid_up_incl_gop, paid_up_excl_gop, vintage_date
			        FROM cdc_penetration_rows
			       WHERE paid_up_incl_gop IS NOT NULL AND paid_up_excl_gop IS NOT NULL`
			var argv []any
			if scoped != "" {
				q += ` AND vintage_date = ?`
				argv = append(argv, scoped)
			}
			if s := strings.TrimSpace(symbol); s != "" {
				q += ` AND (symbol = ? OR isin = ?)`
				argv = append(argv, s, s)
			}
			q += ` ORDER BY (paid_up_incl_gop - paid_up_excl_gop) DESC`
			rows, err := db.DB().QueryContext(ctx, q, argv...)
			if err != nil {
				return apiErr(err)
			}
			for rows.Next() {
				var r gopRow
				var vd string
				if err := rows.Scan(&r.ISIN, &r.Symbol, &r.Name, &r.Status,
					&r.PaidUpIncl, &r.PaidUpExcl, &vd); err != nil {
					_ = rows.Close()
					return apiErr(err)
				}
				_ = vd
				view.Scanned++
				r.GopShares = r.PaidUpIncl - r.PaidUpExcl
				if r.PaidUpIncl > 0 {
					r.GopPct = math.Round(10000*r.GopShares/r.PaidUpIncl) / 100
				}
				if r.GopShares <= 0 && minPct > 0 {
					continue
				}
				if r.GopPct < minPct {
					continue
				}
				view.WithGop++
				if limit > 0 && len(view.Rows) >= limit {
					continue
				}
				view.Rows = append(view.Rows, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return apiErr(err)
			}
			if err := rows.Close(); err != nil {
				return apiErr(err)
			}

			view.Note = fmt.Sprintf("scanned %d rows carrying both capital columns; %d meet the >=%.2f%% threshold (returning %d). UNIVERSE WIDTH IS %d -- read any cross-sectional number against it.",
				view.Scanned, view.WithGop, minPct, len(view.Rows), view.Scanned)
			if view.Scanned == 0 {
				// Distinguish "your filter matched nothing" from "nothing is
				// stored". Reporting the former as the latter sends the operator
				// to run a sync they have already run.
				var total int
				if qerr := db.DB().QueryRowContext(ctx,
					`SELECT COUNT(*) FROM cdc_penetration_rows WHERE paid_up_incl_gop IS NOT NULL`).Scan(&total); qerr != nil {
					total = 0
				}
				if total > 0 {
					filters := []string{}
					if strings.TrimSpace(symbol) != "" {
						filters = append(filters, "--symbol "+symbol)
					}
					if scoped != "" {
						filters = append(filters, "--vintage "+scoped)
					}
					f := strings.Join(filters, " ")
					if f == "" {
						f = "the requested threshold"
					}
					view.Note = fmt.Sprintf("no rows matched your filter (%s). The store holds %d rows with capital columns, so this is an empty FILTER result, NOT an empty store.", f, total)
				} else {
					view.Note = "no rows with both capital columns are stored. Part B (funds) has no share capital, so run `verify rows` on part A of a vintage first."
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if len(view.Rows) == 0 {
				fmt.Fprintln(w, view.Note)
				return nil
			}
			fmt.Fprintf(w, "%-12s %-9s %14s %8s  %s\n", "ISIN", "SYMBOL", "GOP SHARES", "GOP %", "NAME")
			for _, r := range view.Rows {
				fmt.Fprintf(w, "%-12s %-9s %14.0f %7.2f%%  %s\n", r.ISIN, r.Symbol, r.GopShares, r.GopPct, truncStr(r.Name, 40))
			}
			fmt.Fprintf(w, "\n%s\ncaveat: %s\n", view.Note, view.Caveat)
			return nil
		},
	}
	cmd.Flags().StringVar(&vintage, "vintage", "", "restrict to one vintage date")
	cmd.Flags().Float64Var(&minPct, "min-pct", 0, "only securities where GoP holds at least this percent of paid-up capital")
	cmd.Flags().StringVar(&symbol, "symbol", "", "restrict to one symbol or ISIN")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

const gopCaveat = "LEVEL ONLY -- there is exactly one live vintage of the source report, so no time series and no deltas exist. Derived as paid_up_incl_gop minus paid_up_excl_gop from CDC's own columns."

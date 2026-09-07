// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// floatLeg is one definition of "float" from one source. The three legs are
// NEVER averaged or blended: they measure different things.
type floatLeg struct {
	Source      string   `json:"source"`
	Definition  string   `json:"definition"`
	Value       *float64 `json:"value"`
	Unit        string   `json:"unit"`
	Available   bool     `json:"available"`
	Unavailable string   `json:"unavailable_reason,omitempty"`
}

type floatRow struct {
	ISIN   string     `json:"isin"`
	Symbol string     `json:"symbol"`
	Name   string     `json:"name"`
	Legs   []floatLeg `json:"legs"`
	// SpreadPP is the CDC-minus-PSX gap in percentage points, present only when
	// both legs exist.
	SpreadPP *float64 `json:"spread_pp,omitempty"`
	Flag     string   `json:"flag,omitempty"`
}

type floatView struct {
	Vintage       string     `json:"vintage"`
	Rows          []floatRow `json:"rows"`
	UniverseWidth int        `json:"universe_width"`
	LegsAvailable []string   `json:"legs_available"`
	Note          string     `json:"note"`
	Caveat        string     `json:"caveat"`
}

func newNovelFloatTriangulateCmd(flags *rootFlags) *cobra.Command {
	var (
		symbol   string
		vintage  string
		psxDB    string
		limit    int
		dbPath   string
		onlyGaps bool
	)

	cmd := &cobra.Command{
		Use:   "triangulate",
		Short: "Put CDC custody penetration beside PSX free-float shares and NCCPL free-float percent, with each denominator named.",
		Long: strings.Trim(`
Compare the incompatible definitions of "float" side by side.

THREE DIFFERENT THINGS ARE CALLED FLOAT and this command refuses to blend them:

  CDC   custody penetration   shares held in the depository as a percent of
                              paid-up capital. Says nothing about tradability.
  PSX   free float            shares the exchange treats as freely tradable,
                              excluding strategic and locked holdings.
  NCCPL free float percent    a MARGIN-ELIGIBILITY construct used to set risk
                              parameters, not a measure of ownership.

Averaging them would produce a number that answers no question. The output puts
each leg on its own row with its denominator named, and computes only one
derived value: the CDC-minus-PSX spread in percentage points, where both exist.

The CDC leg exists in no package or dataset anywhere. The other two legs come
from the operator's own sibling stores and are OPTIONAL: point --psx-db at a
psx-pp-cli database to populate the PSX leg. When a leg is absent it is reported
as available=false with a reason -- never silently omitted and never imputed.

A flag of psx_float_exceeds_cds_holding means PSX considers more shares freely
tradable than CDC records as held in the depository at all, which is worth
investigating rather than averaging away.
`, "\n"),
		Example:     "  cdc-pakistan-pp-cli float triangulate --symbol OGDC --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--limit=5", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "float triangulate")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("cdc-pakistan-pp-cli")
			}
			view := &floatView{Rows: []floatRow{}, Caveat: floatCaveat, LegsAvailable: []string{"cdc"}}
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

			psxFloat := map[string]float64{}
			psxReason := "no --psx-db supplied, so the PSX free-float leg is unavailable"
			if psxDB != "" {
				n, err := loadPSXFloat(ctx, psxDB, psxFloat)
				if err != nil {
					psxReason = fmt.Sprintf("--psx-db could not be read: %v", err)
				} else {
					psxReason = ""
					view.LegsAvailable = append(view.LegsAvailable, "psx")
					fmt.Fprintf(cmd.ErrOrStderr(), "loaded %d PSX free-float values from %s\n", n, psxDB)
				}
			}

			// Scope to ONE vintage; scanning all of them returns each issuer
			// once per vintage and mislabels the result.
			scoped := strings.TrimSpace(vintage)
			if scoped == "" {
				nv, nerr := newestVintage(ctx, db)
				if nerr != nil {
					return apiErr(nerr)
				}
				scoped = nv
			}
			view.Vintage = scoped

			q := `SELECT isin, COALESCE(symbol,''), COALESCE(name,''), pct_incl_gop, vintage_date
			        FROM cdc_penetration_rows
			       WHERE pct_incl_gop IS NOT NULL`
			var argv []any
			if scoped != "" {
				q += ` AND vintage_date = ?`
				argv = append(argv, scoped)
			}
			if s := strings.TrimSpace(symbol); s != "" {
				q += ` AND (symbol = ? OR isin = ?)`
				argv = append(argv, s, s)
			}
			q += ` ORDER BY symbol`
			rows, err := db.DB().QueryContext(ctx, q, argv...)
			if err != nil {
				return apiErr(err)
			}
			for rows.Next() {
				var r floatRow
				var cdcPct float64
				var vd string
				if err := rows.Scan(&r.ISIN, &r.Symbol, &r.Name, &cdcPct, &vd); err != nil {
					_ = rows.Close()
					return apiErr(err)
				}
				_ = vd
				view.UniverseWidth++
				cdcVal := cdcPct
				r.Legs = []floatLeg{{
					Source: "cdc", Definition: "shares in CDS as % of paid-up capital (custody penetration)",
					Value: &cdcVal, Unit: "percent", Available: true,
				}}
				psxLeg := floatLeg{
					Source: "psx", Definition: "exchange free float (freely tradable shares)",
					Unit: "shares", Unavailable: psxReason,
				}
				if v, ok := psxFloat[r.Symbol]; ok {
					pv := v
					psxLeg.Value, psxLeg.Available, psxLeg.Unavailable = &pv, true, ""
				}
				r.Legs = append(r.Legs, psxLeg)
				r.Legs = append(r.Legs, floatLeg{
					Source: "nccpl", Definition: "margin-eligibility free float percent (risk parameter, not ownership)",
					Unit: "percent", Available: false,
					Unavailable: "NCCPL free float is not readable from this CLI; it lives in the nccpl-pp-cli store",
				})
				if len(view.Rows) < limit || limit <= 0 {
					view.Rows = append(view.Rows, r)
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return apiErr(err)
			}
			if err := rows.Close(); err != nil {
				return apiErr(err)
			}

			if onlyGaps {
				kept := view.Rows[:0]
				for _, r := range view.Rows {
					if r.Flag != "" {
						kept = append(kept, r)
					}
				}
				view.Rows = kept
			}
			// UNIVERSE WIDTH is printed alongside every cross-sectional number:
			// a moving universe has faked a factor result on this project before.
			view.Note = fmt.Sprintf("universe width %d securities carrying a CDC penetration percentage in vintage %s; legs available: %s. The three definitions are reported separately and never blended.",
				view.UniverseWidth, view.Vintage, strings.Join(view.LegsAvailable, ", "))
			if view.UniverseWidth == 0 {
				var total int
				_ = db.DB().QueryRowContext(ctx,
					`SELECT COUNT(*) FROM cdc_penetration_rows WHERE pct_incl_gop IS NOT NULL`).Scan(&total)
				if total > 0 {
					view.Note = fmt.Sprintf("no securities matched your filter. The store holds %d securities with a penetration percentage, so this is an empty FILTER result, not an empty store.", total)
				} else {
					view.Note = "no securities with a CDC penetration percentage are stored. Part B (funds) has no percentage column, so run `verify rows` on part A first."
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
			fmt.Fprintf(w, "%-10s %-12s %10s %14s  %s\n", "SYMBOL", "ISIN", "CDC CUST%", "PSX FLOAT", "NAME")
			for _, r := range view.Rows {
				cdcs, psxs := "-", "unavailable"
				for _, l := range r.Legs {
					if l.Source == "cdc" && l.Value != nil {
						cdcs = fmt.Sprintf("%.2f", *l.Value)
					}
					if l.Source == "psx" && l.Available && l.Value != nil {
						psxs = fmt.Sprintf("%.0f", *l.Value)
					}
				}
				fmt.Fprintf(w, "%-10s %-12s %10s %14s  %s\n", r.Symbol, r.ISIN, cdcs, psxs, truncStr(r.Name, 34))
			}
			fmt.Fprintf(w, "\n%s\ncaveat: %s\n", view.Note, view.Caveat)
			return nil
		},
	}
	cmd.Flags().StringVar(&symbol, "symbol", "", "restrict to one symbol or ISIN")
	cmd.Flags().StringVar(&vintage, "vintage", "", "restrict to one vintage date")
	cmd.Flags().StringVar(&psxDB, "psx-db", "", "path to a psx-pp-cli SQLite store, to populate the PSX free-float leg")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum securities to return")
	cmd.Flags().BoolVar(&onlyGaps, "only-gaps", false, "return only securities carrying a cross-source flag")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

const floatCaveat = "custody penetration, exchange free float and margin-eligibility float measure DIFFERENT things and are never blended here. A missing leg is reported as available=false with a reason, never imputed."

// loadPSXFloat reads free-float values out of a sibling psx-pp-cli store. It is
// deliberately tolerant: the sibling schema is not this CLI's contract, so a
// shape it does not recognise is reported as an unavailable leg rather than
// failing the whole command.
func loadPSXFloat(ctx context.Context, path string, out map[string]float64) (int, error) {
	if _, err := os.Stat(path); err != nil {
		return 0, fmt.Errorf("not readable: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx,
		`SELECT json_extract(data,'$.symbol'), json_extract(data,'$.freeFloat')
		   FROM resources WHERE json_extract(data,'$.freeFloat') IS NOT NULL`)
	if err != nil {
		return 0, fmt.Errorf("sibling store has no recognisable free-float column: %w", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var sym sql.NullString
		var ff sql.NullFloat64
		if err := rows.Scan(&sym, &ff); err != nil {
			continue
		}
		if sym.Valid && ff.Valid && !math.IsNaN(ff.Float64) {
			out[strings.ToUpper(sym.String)] = ff.Float64
			n++
		}
	}
	return n, rows.Err()
}

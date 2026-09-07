// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type vintageFingerprint struct {
	Vintage     string  `json:"vintage_date"`
	Part        string  `json:"part"`
	Layout      string  `json:"layout"`
	Title       string  `json:"title"`
	Producer    string  `json:"producer"`
	Pages       int     `json:"pages"`
	Rows        int     `json:"rows_parsed"`
	CoveragePct float64 `json:"coverage_pct"`
	Fingerprint string  `json:"schema_fingerprint"`
}

type schemaView struct {
	Vintages         []vintageFingerprint `json:"vintages"`
	DistinctFPs      int                  `json:"distinct_fingerprints"`
	Drifts           []string             `json:"drift"`
	IncompleteMonths []string             `json:"incomplete_months"`
	Note             string               `json:"note"`
}

func newNovelVerifySchemaCmd(flags *rootFlags) *cobra.Command {
	var vintage string
	var acceptFP string
	var strict bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Refuse to blend vintages whose column sets are incompatible, and refuse a month that is missing one of its parts.",
		Long: strings.Trim(`
Fingerprint each stored vintage and report schema drift.

Drift is not hypothetical here. CDC changed its production pipeline between the
two vintages that can actually be compared:

                       2024-01-31            2025-11-30 part A
  producer             Microsoft Excel 2016  Microsoft: Print To PDF
  rows                 1,206                 29,350
  Market Value column  PRESENT               GONE
  STATUS column        absent                PRESENT
  percentage column    absent                PRESENT

That is an incompatible column set and a twenty-four-fold universe change, so
those vintages cannot be differenced. This command exists to make that refusal
explicit rather than letting a blend produce a plausible, wrong series.

It also reports a month missing one of its PARTS. The report is split by
instrument class -- part A is ordinary/preference/modaraba shares, part B is
open-end/ETF funds and saving certificates -- and the parts are DISJOINT, so
either alone is a partial slice of the month.
`, "\n"),
		Example:     "  cdc-pakistan-pp-cli verify schema --vintage 2025-11-30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--vintage=2025-11-30", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "verify schema")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("cdc-pakistan-pp-cli")
			}
			view := &schemaView{Vintages: []vintageFingerprint{}, Drifts: []string{}, IncompleteMonths: []string{}}
			db, ready, oerr := openCDCMirror(ctx, dbPath, "cdc_vintages")
			if oerr != nil {
				return oerr
			}
			if !ready {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: cdc-pakistan-pp-cli verify rows --vintage DATE --db %s\n", dbPath, dbPath)
				view.Note = "no vintages stored yet"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}
			defer db.Close()

			q := `SELECT vintage_date, part, COALESCE(layout,''), COALESCE(title,''),
			             COALESCE(producer,''), COALESCE(pages,0), COALESCE(rows_parsed,0),
			             COALESCE(coverage_pct,0), COALESCE(schema_fingerprint,'')
			        FROM cdc_vintages`
			var argv []any
			if v := strings.TrimSpace(vintage); v != "" {
				q += ` WHERE vintage_date = ?`
				argv = append(argv, v)
			}
			q += ` ORDER BY vintage_date DESC, part`
			rows, err := db.DB().QueryContext(ctx, q, argv...)
			if err != nil {
				return apiErr(err)
			}
			fps := map[string]string{}
			partsByMonth := map[string]map[string]bool{}
			for rows.Next() {
				var f vintageFingerprint
				if err := rows.Scan(&f.Vintage, &f.Part, &f.Layout, &f.Title, &f.Producer,
					&f.Pages, &f.Rows, &f.CoveragePct, &f.Fingerprint); err != nil {
					_ = rows.Close()
					return apiErr(err)
				}
				view.Vintages = append(view.Vintages, f)
				// Key drift detection on the PART, not the layout. Parts A and B
				// of one month legitimately carry DIFFERENT layouts -- A is
				// ordinary/preference/modaraba shares, B is open-end/ETF funds
				// with no capital columns at all. Treating that as drift called
				// one healthy month "2 incompatible layouts".
				if prev, ok := fps[f.Part]; ok && prev != f.Fingerprint {
					view.Drifts = append(view.Drifts,
						fmt.Sprintf("part %s has two schema fingerprints across vintages: %q and %q -- these vintages are not differenceable", f.Part, prev, f.Fingerprint))
				}
				fps[f.Part] = f.Fingerprint
				if partsByMonth[f.Vintage] == nil {
					partsByMonth[f.Vintage] = map[string]bool{}
				}
				partsByMonth[f.Vintage][f.Part] = true
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return apiErr(err)
			}
			if err := rows.Close(); err != nil {
				return apiErr(err)
			}

			// Cross-vintage drift WITHIN one part is the real signal. Compare
			// each part's layout across vintages; differing layouts BETWEEN
			// parts of the same month are expected and are not drift.
			layoutsByPart := map[string]map[string]bool{}
			for _, v := range view.Vintages {
				if layoutsByPart[v.Part] == nil {
					layoutsByPart[v.Part] = map[string]bool{}
				}
				layoutsByPart[v.Part][v.Layout] = true
			}
			for part, ls := range layoutsByPart {
				if len(ls) <= 1 {
					continue
				}
				names := make([]string, 0, len(ls))
				for l := range ls {
					names = append(names, l)
				}
				sort.Strings(names)
				view.Drifts = append(view.Drifts, fmt.Sprintf(
					"part %s spans %d incompatible layouts across vintages (%s); those vintages must NOT be differenced into a series",
					part, len(ls), strings.Join(names, ", ")))
			}
			for month, parts := range partsByMonth {
				// A month holding only part A or only part B is incomplete,
				// because the parts are disjoint instrument classes.
				if len(parts) == 1 && (parts["A"] || parts["B"]) {
					have := "A"
					if parts["B"] {
						have = "B"
					}
					view.IncompleteMonths = append(view.IncompleteMonths, fmt.Sprintf(
						"%s has only part %s; the parts are disjoint instrument classes so this month is a PARTIAL SLICE", month, have))
				}
			}
			uniq := map[string]bool{}
			for _, v := range view.Vintages {
				uniq[v.Fingerprint] = true
			}
			view.DistinctFPs = len(uniq)

			switch {
			case len(view.Vintages) == 0:
				view.Note = "no vintages stored; run `verify rows --vintage <date>` first"
			case len(view.Drifts) == 0 && len(view.IncompleteMonths) == 0:
				view.Note = fmt.Sprintf("%d vintage part(s), %d distinct schema fingerprint(s), no drift and no partial months detected", len(view.Vintages), view.DistinctFPs)
			default:
				view.Note = fmt.Sprintf("%d drift finding(s) and %d partial month(s): these vintages are NOT safe to difference", len(view.Drifts), len(view.IncompleteMonths))
			}
			if acceptFP != "" {
				view.Note += fmt.Sprintf(" (fingerprint %q acknowledged by the operator)", acceptFP)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "%-12s %-5s %-14s %8s %9s %8s\n", "VINTAGE", "PART", "LAYOUT", "PAGES", "ROWS", "COVER%")
				for _, v := range view.Vintages {
					fmt.Fprintf(w, "%-12s %-5s %-14s %8d %9d %7.2f\n", v.Vintage, v.Part, v.Layout, v.Pages, v.Rows, v.CoveragePct)
				}
				for _, d := range view.Drifts {
					fmt.Fprintf(w, "\nDRIFT: %s\n", d)
				}
				for _, m := range view.IncompleteMonths {
					fmt.Fprintf(w, "PARTIAL: %s\n", m)
				}
				fmt.Fprintf(w, "\n%s\n", view.Note)
			}
			if strict && acceptFP == "" && (len(view.Drifts) > 0 || len(view.IncompleteMonths) > 0) {
				return notFoundErr(fmt.Errorf("--strict: schema drift or partial months present; acknowledge with --accept-schema to proceed"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&vintage, "vintage", "", "restrict to one vintage date")
	cmd.Flags().StringVar(&acceptFP, "accept-schema", "", "acknowledge a specific fingerprint so --strict passes")
	cmd.Flags().BoolVar(&strict, "strict", false, "exit non-zero on unacknowledged drift or a partial month")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

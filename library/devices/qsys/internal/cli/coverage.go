// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type coverageSource struct {
	Source      string  `json:"source"`
	Attempted   int     `json:"attempted"`
	Succeeded   int     `json:"succeeded"`
	WithSpecs   int     `json:"with_spec_sheet,omitempty"`
	Rate        float64 `json:"success_rate"`
	LastError   string  `json:"last_error,omitempty"`
	FinishedAt  string  `json:"finished_at,omitempty"`
}

type coverageReport struct {
	Sources       []coverageSource `json:"sources"`
	StoredPages    int             `json:"stored_pages"`
	StoredProducts int             `json:"stored_products"`
	StoredCompat   int             `json:"stored_compat_rows"`
	SpecTextRows   int             `json:"products_with_spec_text"`
	Note           string          `json:"note,omitempty"`
}

func newNovelCoverageCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Report how many products resolved a spec sheet and how many pages parsed, so extraction gaps are visible.",
		Long: strings.Trim(`
Coverage reports what the last harvest actually captured from each vendor site.

This exists because both sources are scraped, not served from an API. When QSC
changes their HTML, extraction degrades silently: commands keep exiting zero and
simply return less. A success rate printed as a number turns that into something
you can see.

Run it after every harvest. A sudden drop means the vendor changed their markup.
`, "\n"),
		Example:     "  qsys-pp-cli coverage --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "coverage")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), coverageReport{Sources: make([]coverageSource, 0)}, flags)
				}
				return nil
			}
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			db := st.DB()

			rep := coverageReport{Sources: make([]coverageSource, 0, 3)}

			rows, err := db.QueryContext(ctx,
				`SELECT source, attempted, succeeded, with_specs, last_error, finished_at
				 FROM qsys_harvest ORDER BY source`)
			if err != nil {
				return fmt.Errorf("reading harvest stats: %w", err)
			}
			for rows.Next() {
				var s coverageSource
				var lastErr, finished sql.NullString
				if err := rows.Scan(&s.Source, &s.Attempted, &s.Succeeded, &s.WithSpecs, &lastErr, &finished); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning harvest stats: %w", err)
				}
				s.LastError = lastErr.String
				s.FinishedAt = finished.String
				if s.Attempted > 0 {
					s.Rate = float64(s.Succeeded) / float64(s.Attempted)
				}
				rep.Sources = append(rep.Sources, s)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating harvest stats: %w", err)
			}
			if err := rows.Close(); err != nil {
				return err
			}

			rep.StoredPages, err = countRows(ctx, db, `SELECT COUNT(*) FROM qsys_pages`)
			if err != nil {
				return err
			}
			rep.StoredProducts, err = countRows(ctx, db, `SELECT COUNT(*) FROM qsys_products`)
			if err != nil {
				return err
			}
			rep.StoredCompat, err = countRows(ctx, db, `SELECT COUNT(*) FROM qsys_compat`)
			if err != nil {
				return err
			}
			rep.SpecTextRows, err = countRows(ctx, db, `SELECT COUNT(*) FROM qsys_products WHERE spec_text != ''`)
			if err != nil {
				return err
			}

			if len(rep.Sources) == 0 {
				rep.Note = "no harvest recorded yet; run: qsys-pp-cli harvest"
			} else if rep.StoredProducts > 0 && rep.SpecTextRows == 0 {
				rep.Note = "no spec-sheet text extracted; re-run: qsys-pp-cli harvest --only products --with-pdfs"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-10s %9s %9s %7s\n", "SOURCE", "ATTEMPT", "OK", "RATE")
			for _, s := range rep.Sources {
				fmt.Fprintf(cmd.OutOrStdout(), "%-10s %9d %9d %6.0f%%\n", s.Source, s.Attempted, s.Succeeded, s.Rate*100)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"\nstored: %d pages, %d products (%d with spec text), %d compat rows\n",
				rep.StoredPages, rep.StoredProducts, rep.SpecTextRows, rep.StoredCompat)
			if rep.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", rep.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Corpus database path")
	return cmd
}

func countRows(ctx context.Context, db *sql.DB, query string) (int, error) {
	var n int
	if err := db.QueryRowContext(ctx, query).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

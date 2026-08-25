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

type connectResult struct {
	Model         string        `json:"model"`
	Family        string        `json:"family,omitempty"`
	ModelSpecific []relatedPage `json:"model_specific_pages"`
	General       []relatedPage `json:"general_networking_pages"`
	ScannedPages  int           `json:"scanned_pages"`
	Note          string        `json:"note,omitempty"`
}

func newNovelConnectCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "connect [model]",
		Short: "Get the networking, wiring, and I/O guidance that actually applies to a given model.",
		Long: strings.Trim(`
Connect answers "how do I wire this in" for a specific model.

The Q-SYS Help networking section is a flat set of pages covering addressing,
clocking, Dante, QoS, multicast, switch infrastructure, and discovery. None of
it is indexed per device, so today you read the section and decide what applies.

This command splits the answer in two: pages that actually mention the model,
and the general networking pages that apply to any Q-SYS deployment. The second
list is always returned, because for most models the general guidance IS the
answer and an empty result would be misleading.

Use 'product get' instead when you want specifications rather than wiring.
`, "\n"),
		Example: strings.Trim(`
  qsys-pp-cli connect TSC-70-G3 --agent
  qsys-pp-cli connect CX-Q --limit 10
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true", "pp:happy-args": "model=TSC-70-G3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "connect")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a product model is required, e.g. TSC-70-G3"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if limit <= 0 {
				limit = 8
			}
			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), connectResult{
						Model:         args[0],
						ModelSpecific: make([]relatedPage, 0),
						General:       make([]relatedPage, 0),
					}, flags)
				}
				return nil
			}
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			db := st.DB()

			model := args[0]
			res := connectResult{
				Model:         model,
				ModelSpecific: make([]relatedPage, 0, limit),
				General:       make([]relatedPage, 0, limit),
			}
			if p, found, err := findProduct(ctx, db, model); err != nil {
				return err
			} else if found {
				res.Model, res.Family = p.Model, p.Family
			}
			res.ScannedPages, err = countRows(ctx, db,
				`SELECT COUNT(*) FROM qsys_pages WHERE section IN ('Networking','Connect','Hardware')`)
			if err != nil {
				return err
			}

			res.ModelSpecific, err = pageQuery(ctx, db, `
				SELECT title, section, url FROM qsys_pages
				WHERE section IN ('Networking','Connect','Hardware')
				  AND (lower(title) LIKE '%'||?||'%' OR lower(body) LIKE '%'||?||'%')
				ORDER BY length(title) LIMIT ?`,
				strings.ToLower(res.Model), strings.ToLower(res.Model), limit)
			if err != nil {
				return err
			}
			res.General, err = pageQuery(ctx, db, `
				SELECT title, section, url FROM qsys_pages
				WHERE section IN ('Networking','Connect')
				ORDER BY length(title) LIMIT ?`, limit)
			if err != nil {
				return err
			}

			switch {
			case res.ScannedPages == 0:
				res.Note = "no networking pages harvested; run `qsys-pp-cli harvest --only pages`"
			case len(res.ModelSpecific) == 0:
				res.Note = fmt.Sprintf("no page mentions %q by name; the general networking guidance below applies to any Q-SYS device", res.Model)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Connection guidance for %s\n", res.Model)
			printPages(cmd.OutOrStdout(), "MENTIONS THIS MODEL", res.ModelSpecific)
			printPages(cmd.OutOrStdout(), "GENERAL Q-SYS NETWORKING", res.General)
			if res.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nnote: %s\n", res.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Corpus database path")
	cmd.Flags().IntVar(&limit, "limit", 8, "Maximum pages per list")
	return cmd
}

func pageQuery(ctx context.Context, db *sql.DB, query string, args ...any) ([]relatedPage, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying pages: %w", err)
	}
	defer rows.Close()
	out := make([]relatedPage, 0)
	for rows.Next() {
		var r relatedPage
		var title, section sql.NullString
		if err := rows.Scan(&title, &section, &r.URL); err != nil {
			return nil, fmt.Errorf("scanning page: %w", err)
		}
		r.Title, r.Section = title.String, section.String
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pages: %w", err)
	}
	return out, nil
}

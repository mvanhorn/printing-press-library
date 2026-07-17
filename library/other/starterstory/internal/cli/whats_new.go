// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: rows first seen on the most recent index run (a diff against
// the previous run).

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/starterstory/internal/store"
	"github.com/spf13/cobra"
)

type whatsNewRow struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Section string `json:"section"`
	Revenue int64  `json:"revenue"`
	URL     string `json:"url"`
}

// pp:data-source local
func newNovelWhatsNewCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagSection string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "whats-new",
		Short:       "Diff the sitemap against your last sync to surface newly published stories, ideas, and businesses.",
		Example:     "  starterstory-pp-cli whats-new --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("starterstory-pp-cli")
			}
			if missingIndexGuard(cmd, flags, dbPath) {
				return nil
			}

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()
			if err := db.EnsureStarterStoryIndex(ctx); err != nil {
				return err
			}

			lastRun, err := db.StarterStoryLastRun(ctx)
			if err != nil {
				return err
			}
			if lastRun == "" {
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"results": []whatsNewRow{},
						"note":    "run index at least once",
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "run index at least once")
				return nil
			}

			sqlText := `SELECT slug, title, section, revenue, url FROM ss_index WHERE first_seen = ?`
			sqlArgs := []any{lastRun}
			if flagSection != "" {
				sqlText += " AND section = ?"
				sqlArgs = append(sqlArgs, flagSection)
			}
			sqlText += " ORDER BY revenue DESC, slug LIMIT ?"
			sqlArgs = append(sqlArgs, flagLimit)

			rows, err := db.DB().QueryContext(ctx, sqlText, sqlArgs...)
			if err != nil {
				return fmt.Errorf("querying index: %w", err)
			}
			out := make([]whatsNewRow, 0, flagLimit)
			for rows.Next() {
				var slug, title, section, url sql.NullString
				var revenue sql.NullInt64
				if err := rows.Scan(&slug, &title, &section, &revenue, &url); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning row: %w", err)
				}
				out = append(out, whatsNewRow{
					Slug:    slug.String,
					Title:   title.String,
					Section: section.String,
					Revenue: revenue.Int64,
					URL:     url.String,
				})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating rows: %w", err)
			}
			_ = rows.Close()

			if flags.asJSON || flags.agent {
				envelope := map[string]any{
					"results":  out,
					"last_run": lastRun,
					"count":    len(out),
				}
				if len(out) == 0 {
					envelope["note"] = "no new rows since the previous index run"
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no new rows since the previous index run")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SECTION\tREVENUE/MO\tSLUG\tTITLE")
			for _, r := range out {
				fmt.Fprintf(tw, "%s\t$%d\t%s\t%s\n", r.Section, r.Revenue, r.Slug, truncate(r.Title, 55))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum number of new rows to return")
	cmd.Flags().StringVar(&flagSection, "section", "", "Filter to a single section")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

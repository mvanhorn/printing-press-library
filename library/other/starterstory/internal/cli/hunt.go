// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: filtered search over the local index (slug/title LIKE, with
// section and minimum-revenue filters).

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/starterstory/internal/store"
	"github.com/spf13/cobra"
)

type huntRow struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Revenue int64  `json:"revenue"`
	Section string `json:"section"`
	URL     string `json:"url"`
}

// pp:data-source local
func newNovelHuntCmd(flags *rootFlags) *cobra.Command {
	var flagSection string
	var flagMinRevenue int
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "hunt <query>",
		Short:       "Full-text search the local index with section and minimum-revenue filters (hunt = filtered search).",
		Example:     "  starterstory-pp-cli hunt saas --section stories --min-revenue 20000",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				return usageErr(fmt.Errorf("query is required\nUsage: %s <query>", cmd.CommandPath()))
			}
			query := args[0]

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

			sqlText := `SELECT slug, title, revenue, section, url FROM ss_index WHERE (slug LIKE ? OR title LIKE ?)`
			like := "%" + query + "%"
			sqlArgs := []any{like, like}
			if flagSection != "" {
				sqlText += " AND section = ?"
				sqlArgs = append(sqlArgs, flagSection)
			}
			if flagMinRevenue > 0 {
				sqlText += " AND revenue >= ?"
				sqlArgs = append(sqlArgs, flagMinRevenue)
			}
			sqlText += " ORDER BY revenue DESC, slug LIMIT ?"
			sqlArgs = append(sqlArgs, flagLimit)

			rows, err := db.DB().QueryContext(ctx, sqlText, sqlArgs...)
			if err != nil {
				return fmt.Errorf("querying index: %w", err)
			}
			out := make([]huntRow, 0, flagLimit)
			for rows.Next() {
				var slug, title, section, url sql.NullString
				var revenue sql.NullInt64
				if err := rows.Scan(&slug, &title, &revenue, &section, &url); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning row: %w", err)
				}
				out = append(out, huntRow{
					Slug:    slug.String,
					Title:   title.String,
					Revenue: revenue.Int64,
					Section: section.String,
					URL:     url.String,
				})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating rows: %w", err)
			}
			_ = rows.Close()

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no matches for %q\n", query)
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
	cmd.Flags().StringVar(&flagSection, "section", "", "Filter to a single section (stories, ideas, businesses, breakdowns, tools, data)")
	cmd.Flags().IntVar(&flagMinRevenue, "min-revenue", 0, "Only include rows with revenue >= this many dollars/month")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum number of matches to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

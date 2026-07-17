// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: regexp scan across the local index slugs and titles.

package cli

import (
	"database/sql"
	"fmt"
	"regexp"

	"github.com/mvanhorn/printing-press-library/library/other/starterstory/internal/store"
	"github.com/spf13/cobra"
)

type grepRow struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Section string `json:"section"`
	Revenue int64  `json:"revenue"`
	URL     string `json:"url"`
}

// pp:data-source local
func newNovelGrepCmd(flags *rootFlags) *cobra.Command {
	var flagSection string
	var flagLimit int
	var flagMaxScan int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "grep <term>",
		Short:       "Keyword-filter across all indexed idea and story slugs offline.",
		Example:     "  starterstory-pp-cli grep newsletter",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				return usageErr(fmt.Errorf("term is required\nUsage: %s <term>", cmd.CommandPath()))
			}
			re, err := regexp.Compile("(?i)" + args[0])
			if err != nil {
				return usageErr(fmt.Errorf("invalid regexp %q: %w", args[0], err))
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

			sqlText := `SELECT slug, title, revenue, section, url FROM ss_index`
			var sqlArgs []any
			if flagSection != "" {
				sqlText += " WHERE section = ?"
				sqlArgs = append(sqlArgs, flagSection)
			}
			sqlText += " ORDER BY revenue DESC, slug LIMIT ?"
			sqlArgs = append(sqlArgs, flagMaxScan)

			rows, err := db.DB().QueryContext(ctx, sqlText, sqlArgs...)
			if err != nil {
				return fmt.Errorf("querying index: %w", err)
			}
			out := make([]grepRow, 0)
			scanned := 0
			for rows.Next() {
				var slug, title, section, url sql.NullString
				var revenue sql.NullInt64
				if err := rows.Scan(&slug, &title, &revenue, &section, &url); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning row: %w", err)
				}
				scanned++
				if !re.MatchString(slug.String) && !re.MatchString(title.String) {
					continue
				}
				out = append(out, grepRow{
					Slug:    slug.String,
					Title:   title.String,
					Section: section.String,
					Revenue: revenue.Int64,
					URL:     url.String,
				})
				if len(out) >= flagLimit {
					break
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating rows: %w", err)
			}
			_ = rows.Close()

			if flags.asJSON || flags.agent {
				envelope := map[string]any{
					"results": out,
					"scanned": scanned,
					"matched": len(out),
				}
				if len(out) == 0 {
					envelope["note"] = fmt.Sprintf("no matches for /%s/ across %d scanned rows", args[0], scanned)
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no matches for /%s/ across %d scanned rows\n", args[0], scanned)
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
	cmd.Flags().StringVar(&flagSection, "section", "", "Restrict the scan to a single section")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum number of matches to return")
	cmd.Flags().IntVar(&flagMaxScan, "max-scan", 20000, "Maximum number of index rows to scan")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

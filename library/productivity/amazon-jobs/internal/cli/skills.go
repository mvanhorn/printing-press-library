// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: rank teams/cities by demand for a skill keyword.
// Preserved across `generate --force`.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/internal/store"
)

var skillsFacets = map[string]string{
	"team":     "COALESCE(NULLIF(json_extract(data, '$.team.label'), ''), '(unknown)')",
	"city":     "COALESCE(NULLIF(city, ''), '(unknown)')",
	"category": "COALESCE(NULLIF(job_category, ''), '(unknown)')",
}

type skillsView struct {
	Keyword      string      `json:"keyword"`
	By           string      `json:"by"`
	TotalMatches int         `json:"total_matches"`
	Groups       []statGroup `json:"groups"`
}

func newNovelSkillsCmd(flags *rootFlags) *cobra.Command {
	var by, dbFlag string
	var limit int

	cmd := &cobra.Command{
		Use:   "skills <keyword>",
		Short: "Rank teams/cities by how many synced reqs demand a skill keyword",
		Long: strings.Trim(`
Scan the qualification text of the locally-synced jobs for a keyword and rank
teams (or cities) by how many reqs demand it — cross-field demand no single API
call returns.

Run 'sync' first to populate the store. Use 'skills' to rank demand for a
keyword; use 'find' to retrieve the actual reqs and 'stats' to count by facet.`, "\n"),
		Example: strings.Trim(`
  amazon-jobs-pp-cli skills Rust
  amazon-jobs-pp-cli skills Kubernetes --by city --agent`, "\n"),
		// no-error-path-probe: a keyword that no synced req demands is a valid
		// zero-match result (total_matches: 0), not an error — indistinguishable
		// from a rare-but-real skill that simply is not in the local store.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan qualification text for a keyword")
				return nil
			}
			if err := guardDataSource(flags, "local"); err != nil {
				return err
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a skill keyword is required"))
			}
			keyword := strings.TrimSpace(strings.Join(args, " "))
			if keyword == "" {
				return usageErr(fmt.Errorf("a skill keyword is required"))
			}
			if by == "" {
				by = "team"
			}
			by = strings.ToLower(strings.TrimSpace(by))
			expr, ok := skillsFacets[by]
			if !ok {
				return usageErr(fmt.Errorf("invalid --by %q: choose one of team, city, category", by))
			}
			if limit < 1 {
				limit = 20
			}
			dbPath := resolveDBPath(dbFlag)
			if storeMissing(cmd, flags, dbPath) {
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			// LIKE-scan is a substring match; wrap the keyword in a token
			// boundary of common separators is overkill here — qualification
			// text is prose, so a case-insensitive substring is the honest
			// contract.
			like := "%" + strings.ToLower(keyword) + "%"
			whereClause := `(LOWER(basic_qualifications) LIKE ? OR LOWER(preferred_qualifications) LIKE ? OR LOWER(description) LIKE ?)`

			// True total matching reqs (denominator), independent of top-N groups.
			var total int
			if err := db.DB().QueryRowContext(ctx,
				`SELECT COUNT(*) FROM postings WHERE `+whereClause, like, like, like).Scan(&total); err != nil {
				return fmt.Errorf("counting matches: %w", err)
			}

			query := fmt.Sprintf(
				`SELECT %s AS k, COUNT(*) AS n FROM postings WHERE %s GROUP BY k ORDER BY n DESC, k ASC LIMIT ?`,
				expr, whereClause)
			rows, err := db.DB().QueryContext(ctx, query, like, like, like, limit)
			if err != nil {
				return fmt.Errorf("scanning skills: %w", err)
			}
			groups := make([]statGroup, 0, limit)
			for rows.Next() {
				var k sql.NullString
				var n int
				if err := rows.Scan(&k, &n); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning group: %w", err)
				}
				key := k.String
				if !k.Valid || key == "" {
					key = "(unknown)"
				}
				groups = append(groups, statGroup{Key: key, Count: n})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating groups: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing rows: %w", err)
			}

			view := skillsView{Keyword: keyword, By: by, TotalMatches: total, Groups: groups}
			return emitResult(cmd, flags, view, func(w io.Writer) {
				if len(groups) == 0 {
					fmt.Fprintf(w, "no synced reqs mention %q (or the store is empty; run: amazon-jobs-pp-cli sync --max-pages 5)\n", keyword)
					return
				}
				fmt.Fprintf(w, "%d synced req(s) demand %q, by %s:\n\n", total, keyword, by)
				for _, g := range groups {
					fmt.Fprintf(w, "%6d  %s\n", g.Count, g.Key)
				}
			})
		},
	}

	cmd.Flags().StringVar(&by, "by", "team", "Facet to rank by: team, city, category")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum groups to return")
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local SQLite store path (default: platform data dir)")

	return cmd
}

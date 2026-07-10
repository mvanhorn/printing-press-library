// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/awwwards"
)

type topRow struct {
	Rank         int      `json:"rank"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Award        string   `json:"award,omitempty"`
	Score        float64  `json:"score"`
	Overall      float64  `json:"overall"`
	Tags         []string `json:"tags"`
	ThumbnailURL string   `json:"thumbnail_url"`
	DetailURL    string   `json:"detail_url"`
}

type topResult struct {
	Dimension string   `json:"dimension"`
	Items     []topRow `json:"items"`
	Scored    int      `json:"scored_sites_in_mirror"`
	Note      string   `json:"note,omitempty"`
}

func newTopCmd(flags *rootFlags) *cobra.Command {
	var dim string
	var tag string
	var award string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "top",
		Short: "Rank mirrored winners by any jury dimension: design, usability, creativity, content, or overall",
		Long: strings.Trim(`
Top ranks detail-synced sites in the local mirror by a jury score dimension.
Scores come from detail pages, so run 'mirror --details' first.

Use this command to rank individual sites by jury score. Do NOT use it for
aggregate frequency metrics; use 'trends' instead.
`, "\n"),
		Example: strings.Trim(`
  awwwards-pp-cli top --dim design --limit 10
  awwwards-pp-cli top --dim usability --tag e-commerce --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--dim=overall;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank mirrored sites by jury score")
				return nil
			}
			if err := rejectLiveDataSource(flags, "top"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			col, err := scoreColumn(dim)
			if err != nil {
				return err
			}
			if limit < 0 {
				limit = 0
			}

			if dbPath == "" {
				dbPath = defaultDBPath("awwwards-pp-cli")
			}
			db, done := requireMirror(cmd, flags, dbPath)
			if done {
				return nil
			}
			defer db.Close()

			var where []string
			var queryArgs []any
			where = append(where, col+` IS NOT NULL`)
			if tag != "" {
				where = append(where, `s.slug IN (SELECT slug FROM aw_site_tags WHERE tag COLLATE NOCASE IN (?, ?, ?))`)
				queryArgs = append(queryArgs, tag, strings.ReplaceAll(tag, "-", " "), strings.ReplaceAll(tag, "-", "."))
			}
			if award != "" {
				where = append(where, `s.award LIKE '%' || ? || '%'`)
				queryArgs = append(queryArgs, award)
			}
			queryArgs = append(queryArgs, limit)

			q := `SELECT s.slug, s.title, s.award, s.` + col + `, COALESCE(s.score_overall, 0), s.thumbnail,
			             COALESCE((SELECT GROUP_CONCAT(tag, '||') FROM aw_site_tags t WHERE t.slug = s.slug), '')
			      FROM aw_sites s
			      WHERE ` + strings.Join(where, " AND ") + `
			      ORDER BY s.` + col + ` DESC, s.created_at DESC LIMIT ?`

			rows, err := db.DB().QueryContext(ctx, q, queryArgs...)
			if err != nil {
				return fmt.Errorf("querying mirror: %w", err)
			}
			items := make([]topRow, 0)
			for rows.Next() {
				var r topRow
				var thumb, tagsCat string
				var score sql.NullFloat64
				if err := rows.Scan(&r.Slug, &r.Title, &r.Award, &score, &r.Overall, &thumb, &tagsCat); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning row: %w", err)
				}
				r.Score = score.Float64
				r.Rank = len(items) + 1
				r.ThumbnailURL = awwwards.ThumbnailURL(thumb, "")
				r.DetailURL = "https://www.awwwards.com/sites/" + r.Slug
				if tagsCat != "" {
					r.Tags = strings.Split(tagsCat, "||")
				} else {
					r.Tags = []string{}
				}
				items = append(items, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing rows: %w", err)
			}

			var scored int
			if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM aw_sites WHERE score_overall IS NOT NULL`).Scan(&scored); err != nil {
				return fmt.Errorf("counting scored sites: %w", err)
			}

			res := topResult{Dimension: strings.ToLower(dim), Items: items, Scored: scored}
			if res.Dimension == "" {
				res.Dimension = "overall"
			}
			if len(items) == 0 {
				res.Note = fmt.Sprintf("no scored sites match; %d sites in the mirror have scores - run 'awwwards-pp-cli mirror --details' to sync more", scored)
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}

	cmd.Flags().StringVar(&dim, "dim", "overall", "Score dimension: design, usability, creativity, content, or overall")
	cmd.Flags().StringVar(&tag, "tag", "", "Restrict to sites carrying this tag")
	cmd.Flags().StringVar(&award, "award", "", "Restrict to an award tier substring (SOTD, Nominee, ...)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum results")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

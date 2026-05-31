// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type postRanked struct {
	PostID      string `json:"post_id"`
	Title       string `json:"title"`
	Publication string `json:"publication_id"`
	PublishedAt string `json:"published_at"`
	Views       int    `json:"views"`
	Likes       int    `json:"likes"`
	Comments    int    `json:"comments"`
	Restacks    int    `json:"restacks"`
	Score       int    `json:"score"`
	Paywalled   bool   `json:"paywalled"`
}

func newPostsBestCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		by          string
		window      string
		crossPub    bool
		publication string
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "best",
		Short: "Rank posts by views, likes, comments, or restacks.",
		Long: `Rank cached posts by chosen engagement metric. Optionally aggregate across every
publication you own to find your overall top performer.`,
		Example: `  # Top posts by restacks in last 30d, all publications
  substack-pp-cli posts best --by restacks --window 30d --cross-pub --json

  # Top 5 in one publication
  substack-pp-cli posts best --by views --publication mypub-en --limit 5`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "posts best"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("substack-pp-cli")
			}
			validBy := map[string]string{
				"views": "views", "likes": "likes", "comments": "comments", "restacks": "restacks",
			}
			column, ok := validBy[strings.ToLower(by)]
			if !ok {
				return usageErr(fmt.Errorf("invalid --by %q (expected: views, likes, comments, restacks)", by))
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()
			db := s.DB()

			// window e.g. "30d", "7d" — fall back to no filter if unrecognized
			q := fmt.Sprintf(`SELECT id, COALESCE(title, ''), COALESCE(publication_id, ''),
			       COALESCE(publish_date, ''), COALESCE(views, 0), COALESCE(likes, 0),
			       COALESCE(comments, 0), COALESCE(restacks, 0), COALESCE(paywalled, 0),
			       COALESCE(%s, 0) AS rank_value
			FROM posts WHERE 1=1`, column)
			args2 := []any{}
			if windowDays := parseWindowDays(window); windowDays > 0 {
				q += fmt.Sprintf(` AND publish_date >= date('now', '-%d days')`, windowDays)
			}
			if publication != "" && !crossPub {
				q += ` AND (publication_id = ? OR publication_id IN (SELECT id FROM publications WHERE subdomain = ?))`
				args2 = append(args2, publication, publication)
			}
			q += ` ORDER BY rank_value DESC LIMIT ?`
			args2 = append(args2, limit)

			rows, err := db.QueryContext(cmd.Context(), q, args2...)
			if err != nil {
				return fmt.Errorf("ranking posts: %w", err)
			}
			defer rows.Close()

			var out []postRanked
			for rows.Next() {
				var r postRanked
				var paywalled int
				if err := rows.Scan(&r.PostID, &r.Title, &r.Publication, &r.PublishedAt,
					&r.Views, &r.Likes, &r.Comments, &r.Restacks, &paywalled, &r.Score); err != nil {
					return err
				}
				r.Paywalled = paywalled != 0
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				raw, _ := json.Marshal(out)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}

			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "No posts found. Run 'substack-pp-cli sync --full' first.")
				return nil
			}
			fmt.Fprintf(w, "%s\n", bold(fmt.Sprintf("Top %d posts by %s (window=%s, cross-pub=%v)", len(out), column, window, crossPub)))
			fmt.Fprintln(w, strings.Repeat("─", 78))
			fmt.Fprintf(w, "%-30s %-14s %10s %8s %8s %8s %s\n", "Title", "Publication", column, "Likes", "Cmts", "Rest", "Paywall")
			fmt.Fprintln(w, strings.Repeat("─", 78))
			for _, p := range out {
				pay := ""
				if p.Paywalled {
					pay = "✓"
				}
				fmt.Fprintf(w, "%-30s %-14s %10d %8d %8d %8d %s\n",
					truncate(p.Title, 30), truncate(p.Publication, 14),
					p.Score, p.Likes, p.Comments, p.Restacks, pay)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&by, "by", "views", "Rank metric: views | likes | comments | restacks")
	cmd.Flags().StringVar(&window, "window", "30d", "Time window (e.g. 7d, 30d, 90d). Use 'all' for no limit.")
	cmd.Flags().BoolVar(&crossPub, "cross-pub", false, "Aggregate across all publications")
	cmd.Flags().StringVar(&publication, "publication", "", "Filter to one publication (subdomain or id)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max results")
	return cmd
}

func parseWindowDays(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "all" || s == "0" {
		return 0
	}
	if !strings.HasSuffix(s, "d") {
		return 0
	}
	n := 0
	for _, c := range s[:len(s)-1] {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

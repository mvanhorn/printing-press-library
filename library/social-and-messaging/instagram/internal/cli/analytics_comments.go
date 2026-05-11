// Copyright 2026 bryanaaron. Licensed under Apache-2.0.
// Hand-authored Phase 3 novel analytics: comments top-commenters + comments strangers.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"instagram-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

// newCommentsAnalyticsCmd returns the `comments` top-level analytics command.
// (Named commentsAnalytics to avoid conflict with any generated comments group.)
func newCommentsAnalyticsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comments",
		Short: "Comment analytics (powered by local SQLite — run sync comments first)",
	}
	cmd.AddCommand(newTopCommentersCmd(flags))
	cmd.AddCommand(newStrangersCmd(flags))
	return cmd
}

func newTopCommentersCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "top-commenters",
		Short: "Rank commenters across all synced posts (superfan detector)",
		Long: `Aggregates comments across all synced posts and ranks commenters by frequency.
This detects your most engaged followers — people who comment on multiple posts.

Requires prior 'sync comments <media-id>' runs for each post you want to include.
Instagram does not offer a cross-post aggregation endpoint; this JOIN is the only
way to get this view.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  instagram-pp-cli comments top-commenters
  instagram-pp-cli comments top-commenters --limit 20 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			query := `
				SELECT
					json_extract(data, '$.user.username') AS username,
					json_extract(data, '$.user.pk')       AS pk,
					COUNT(*)                              AS comment_count
				FROM resources
				WHERE resource_type = 'comments'
				GROUP BY json_extract(data, '$.user.pk')
				ORDER BY comment_count DESC
			`
			if limit > 0 {
				query += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type commenterRow struct {
				Username     string `json:"username"`
				PK           string `json:"pk"`
				CommentCount int64  `json:"comment_count"`
			}
			var commenters []commenterRow
			for rows.Next() {
				var r commenterRow
				var uname, pk *string
				if err := rows.Scan(&uname, &pk, &r.CommentCount); err == nil {
					if uname != nil {
						r.Username = *uname
					}
					if pk != nil {
						r.PK = *pk
					}
					commenters = append(commenters, r)
				}
			}

			if flags.asJSON {
				raw, _ := json.MarshalIndent(map[string]any{
					"count":      len(commenters),
					"commenters": func() any { if commenters == nil { return []commenterRow{} }; return commenters }(),
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Top commenters across synced posts: %d\n\n", len(commenters))
			if len(commenters) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No comments found. Run: instagram-pp-cli sync comments <media-id>")
				return nil
			}
			tableRows := make([][]string, len(commenters))
			for i, c := range commenters {
				tableRows[i] = []string{c.Username, strconv.FormatInt(c.CommentCount, 10), c.PK}
			}
			return flags.printTable(cmd, []string{"USERNAME", "COMMENTS", "PK"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max commenters to show (0 = unlimited)")
	return cmd
}

func newStrangersCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "strangers",
		Short: "Commenters on your posts who are NOT in your followers list (reach beyond audience)",
		Long: `Performs a 3-table JOIN: comments × followers, returning commenters whose pk
does not appear in the synced followers table.

These are people who engaged with your content but do not follow you — the
clearest signal of organic reach beyond your current audience.

Requires prior runs of:
  instagram-pp-cli sync comments <media-id>  (for each post)
  instagram-pp-cli sync followers <your-username>`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  instagram-pp-cli comments strangers
  instagram-pp-cli comments strangers --limit 30 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			query := `
				SELECT
					json_extract(c.data, '$.user.username') AS username,
					json_extract(c.data, '$.user.pk')       AS pk,
					COUNT(*)                                AS comment_count
				FROM resources c
				WHERE c.resource_type = 'comments'
				AND NOT EXISTS (
					SELECT 1 FROM resources f
					WHERE f.resource_type = 'followers'
					AND json_extract(f.data, '$.pk') = json_extract(c.data, '$.user.pk')
				)
				GROUP BY json_extract(c.data, '$.user.pk')
				ORDER BY comment_count DESC
			`
			if limit > 0 {
				query += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type strangerRow struct {
				Username     string `json:"username"`
				PK           string `json:"pk"`
				CommentCount int64  `json:"comment_count"`
			}
			var strangers []strangerRow
			for rows.Next() {
				var r strangerRow
				var uname, pk *string
				if err := rows.Scan(&uname, &pk, &r.CommentCount); err == nil {
					if uname != nil {
						r.Username = *uname
					}
					if pk != nil {
						r.PK = *pk
					}
					strangers = append(strangers, r)
				}
			}

			if flags.asJSON {
				raw, _ := json.MarshalIndent(map[string]any{
					"count":     len(strangers),
					"strangers": func() any { if strangers == nil { return []strangerRow{} }; return strangers }(),
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Commenters who are not in your followers list: %d\n\n", len(strangers))
			if len(strangers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No strangers found. Either everyone who comments follows you, or run sync first.")
				return nil
			}
			tableRows := make([][]string, len(strangers))
			for i, s := range strangers {
				tableRows[i] = []string{s.Username, strconv.FormatInt(s.CommentCount, 10), s.PK}
			}
			return flags.printTable(cmd, []string{"USERNAME", "COMMENTS", "PK"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max strangers to show (0 = unlimited)")
	return cmd
}

// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: offline FTS5 search over the synced comment corpus.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store"
)

type commentHit struct {
	CommentID string `json:"comment_id"`
	PostURL   string `json:"post_url"`
	Text      string `json:"text"`
	ParentID  string `json:"parent_id,omitempty"`
	LikeCount int64  `json:"like_count"`
}

func newNovelCommentsSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Full-text search across synced comments and replies, offline",
		Long: strings.Trim(`
Use this command to search already-synced comment text offline (FTS5, zero
credits). Do NOT use it to fetch new comments from the API; use
'comments thread' or 'comments sweep' instead. For transcripts, use
'transcripts search'.`, "\n"),
		Example: strings.Trim(`
  scrape-creators-pp-cli comments search "refund" --limit 20
  scrape-creators-pp-cli comments search "delivery time" --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "query=mock-query"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search the local comment corpus")
				return nil
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
			}
			if flags.dataSource == "live" {
				return fmt.Errorf("no live equivalent for this command (comments search reads the local corpus); use --data-source local or auto")
			}
			query := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("scrape-creators-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: scrape-creators-pp-cli comments sweep <handle> --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := store.EnsureCommentCorpus(ctx, db.DB()); err != nil {
				return fmt.Errorf("comment corpus migration: %w", err)
			}

			rows, err := db.DB().QueryContext(ctx, `
				SELECT f.comment_id, f.post_url, f.text, c.parent_id, c.like_count
				FROM sc_comments_fts f
				JOIN sc_comments c ON c.comment_id = f.comment_id
				WHERE sc_comments_fts MATCH ?
				ORDER BY rank
				LIMIT ?`, ftsQuote(query), limit)
			if err != nil {
				return fmt.Errorf("comment search: %w", err)
			}
			hits := make([]commentHit, 0, limit)
			for rows.Next() {
				var h commentHit
				if err := rows.Scan(&h.CommentID, &h.PostURL, &h.Text, &h.ParentID, &h.LikeCount); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan hit: %w", err)
				}
				hits = append(hits, h)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate hits: %w", err)
			}
			if err := rows.Close(); err != nil {
				return err
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no matches in the local comment corpus; sync more posts with: scrape-creators-pp-cli comments sweep <handle>")
			}
			return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum matching comments to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ftsQuote wraps the user query so FTS5 treats it as a phrase query without
// stray quotes breaking the MATCH expression.
func ftsQuote(q string) string {
	return `"` + strings.ReplaceAll(q, `"`, `""`) + `"`
}

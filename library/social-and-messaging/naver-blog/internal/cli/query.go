// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written `query` command. Runs a Korean-aware FTS5 MATCH against
// the locally cached posts_fts index (title + body_text + tags +
// nickname columns, unicode61 tokenizer).
//
// Wired as a top-level `query` because `search` is already taken by
// the Gate-1 keyword-search alias for find posts.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/store"
)

func newQueryCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "query <text>",
		Short: "Search the local FTS index for cached Naver posts (Korean-aware).",
		Long: `Run a Korean-aware FTS5 MATCH against the locally cached posts. The index includes title, body_text, tags, and nickname columns and uses the unicode61 tokenizer, so Korean queries work the same as English.

The search is offline: it does NOT hit Naver. Populate the cache first via:
  - naver-blog-pp-cli posts <url>             one post
  - naver-blog-pp-cli bundle <queries-file>   batch
  - naver-blog-pp-cli sync                    full sync (where configured)

Standard FTS5 query syntax applies: bare terms are AND'd, "double quotes" run phrase search, prefix:foo* matches prefixes. Results are ordered by FTS rank.

Empty index returns a helpful hint rather than an empty result.`,
		Example: `  naver-blog-pp-cli query 칠리
  naver-blog-pp-cli query "여성청결제" --limit 50
  naver-blog-pp-cli query 'title:칠리'`,
		Annotations: map[string]string{
			"pp:endpoint":   "query.local",
			"pp:method":     "GET",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			text := strings.TrimSpace(strings.Join(args, " "))
			if text == "" {
				return usageErr(fmt.Errorf("query text must be non-empty"))
			}
			if flagLimit <= 0 {
				flagLimit = 20
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []map[string]any{}, flags)
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			rows, hint, err := runLocalFTS(ctx, text, flagLimit)
			if err != nil {
				return err
			}
			if hint != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), hint)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Max results to return")
	return cmd
}

// runLocalFTS opens the store and delegates to store.SearchPosts. We
// reshape the rows into a thin slice of map[string]any so --select /
// --compact behave the same as on the rest of the CLI.
func runLocalFTS(ctx context.Context, text string, limit int) ([]map[string]any, string, error) {
	dbPath := defaultDBPath("naver-blog-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, "", fmt.Errorf("opening local store: %w", err)
	}
	defer db.Close()
	// Check the index is populated before issuing the MATCH —
	// surfacing "no posts cached" is friendlier than an empty array
	// with no explanation.
	var n int
	if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM "posts"`).Scan(&n); err != nil {
		return nil, "", fmt.Errorf("checking posts table: %w", err)
	}
	if n == 0 {
		hint := "No posts cached yet. Populate first:\n" +
			"  naver-blog-pp-cli posts <url>\n" +
			"  naver-blog-pp-cli bundle <queries.yaml>\n" +
			"  naver-blog-pp-cli sync posts"
		return []map[string]any{}, hint, nil
	}
	raws, err := db.SearchPosts(text, limit)
	if err != nil {
		return nil, "", fmt.Errorf("FTS query: %w", err)
	}
	out := make([]map[string]any, 0, len(raws))
	for _, raw := range raws {
		var row map[string]any
		if err := json.Unmarshal(raw, &row); err != nil {
			// Drop unparseable rows — they shouldn't exist in a
			// healthy store, but a partial corruption shouldn't kill
			// the whole result.
			continue
		}
		out = append(out, row)
	}
	return out, "", nil
}

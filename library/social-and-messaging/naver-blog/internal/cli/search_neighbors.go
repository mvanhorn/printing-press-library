// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written `neighbors` command. For a target hashtag, returns the
// top N hashtags that co-occur in the cached corpus.
//
// Wired as a top-level `neighbors` (with a hidden `hashtag-neighbors`
// alias) because `search` is already a top-level Gate-1 alias for
// keyword search.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/store"
)

// neighborRow is one element of the output: a hashtag and the number
// of cached posts in which it co-occurs with the target tag.
type neighborRow struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

func newNeighborsCmd(flags *rootFlags) *cobra.Command {
	var flagTop int

	cmd := &cobra.Command{
		Use:     "neighbors <#tag>",
		Aliases: []string{"hashtag-neighbors"},
		Short:   "Top hashtags that co-occur with a target tag in the cached corpus.",
		Long: `For a target hashtag, returns the top N hashtags that co-occur with it across the locally cached corpus.

Reads the posts table (populated by 'find hashtag' / 'posts get' / 'bundle' / 'sync'), parses each row's comma-separated tags, and counts how often each non-target tag appears in posts that also carry the target. Results are sorted by descending count.

The '#' prefix on the target tag is optional. Tag matching is case-insensitive and ignores leading '#'.

If no posts contain the tag yet, prints a hint to populate the cache and exits cleanly with an empty result.`,
		Example: `  naver-blog-pp-cli neighbors 칠리
  naver-blog-pp-cli neighbors #여성청결제 --top 30`,
		Annotations: map[string]string{
			"pp:endpoint":   "neighbors.list",
			"pp:method":     "GET",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			target := strings.TrimSpace(strings.TrimPrefix(args[0], "#"))
			if target == "" {
				return usageErr(fmt.Errorf("hashtag must be non-empty"))
			}
			if flagTop <= 0 {
				flagTop = 20
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				// Verify env: don't open the user's real DB. Emit an
				// empty result so the verifier sees structured output.
				return printJSONFiltered(cmd.OutOrStdout(), []neighborRow{}, flags)
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			rows, hint, err := queryNeighbors(ctx, target, flagTop)
			if err != nil {
				return err
			}
			if hint != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), hint)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&flagTop, "top", 20, "Max number of neighbor tags to return")
	return cmd
}

// queryNeighbors opens the local store and reads every row's tags
// column. The substring filter uses SQL LIKE; the precise membership
// test runs in Go because comma-separated tags require splitting.
//
// Returns (rows, hint, err). hint is non-empty when the corpus is
// empty for this tag — the caller surfaces it on stderr so the JSON
// output stays clean.
func queryNeighbors(ctx context.Context, target string, topN int) ([]neighborRow, string, error) {
	dbPath := defaultDBPath("naver-blog-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, "", fmt.Errorf("opening local store: %w", err)
	}
	defer db.Close()
	// LIKE on the substring is cheap and avoids loading every cached
	// row when the target tag is rare. We then double-check tag
	// membership in Go because LIKE 'foo' also matches 'foobar'.
	rows, err := db.Query(
		`SELECT "tags" FROM "posts" WHERE "tags" IS NOT NULL AND "tags" != '' AND lower("tags") LIKE ?`,
		"%"+strings.ToLower(target)+"%",
	)
	if err != nil {
		return nil, "", fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	counts := make(map[string]int)
	postsMatched := 0
	targetLower := strings.ToLower(target)
	for rows.Next() {
		var tags string
		if err := rows.Scan(&tags); err != nil {
			return nil, "", fmt.Errorf("scan: %w", err)
		}
		// Tags column shape: comma-separated. ParseMobilePost stores
		// each tag with its leading '#' stripped (per postparse), so
		// we don't strip again here. Lowercase compare.
		split := splitStoredTags(tags)
		hasTarget := false
		for _, t := range split {
			if strings.ToLower(t) == targetLower {
				hasTarget = true
				break
			}
		}
		if !hasTarget {
			continue
		}
		postsMatched++
		for _, t := range split {
			low := strings.ToLower(t)
			if low == targetLower {
				continue
			}
			counts[t]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("rows: %w", err)
	}
	if postsMatched == 0 {
		hint := fmt.Sprintf(
			"No posts containing #%s are cached locally yet. Populate the cache first:\n"+
				"  naver-blog-pp-cli find hashtag --tags %s\n"+
				"  naver-blog-pp-cli posts <url>   # for each result\n"+
				"  naver-blog-pp-cli bundle <file> # batch alternative",
			target, target,
		)
		return []neighborRow{}, hint, nil
	}
	out := make([]neighborRow, 0, len(counts))
	for tag, n := range counts {
		out = append(out, neighborRow{Tag: tag, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	if topN > 0 && len(out) > topN {
		out = out[:topN]
	}
	return out, "", nil
}

// splitStoredTags reads a comma-separated tag column. The column
// stores tags as ParseMobilePost emits them (one tag per element,
// '#' already stripped). We trim whitespace and skip empties so
// trailing commas don't introduce phantom tags.
func splitStoredTags(s string) []string {
	out := make([]string, 0)
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		t = strings.TrimPrefix(t, "#")
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

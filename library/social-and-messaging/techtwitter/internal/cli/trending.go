// Hand-authored top-level command: trending. A discoverable, offline-capable
// front door to the ranked curated stream (the homepage's headline surface).
// Live-first; with --data-source local it ranks the local mirror by engagement.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/internal/store"
)

func newTrendingCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var window string

	cmd := &cobra.Command{
		Use:   "trending",
		Short: "Ranked trending curated tweets (live, or offline-ranked from the local mirror)",
		Long: "Show the ranked trending curated tweets — the homepage's headline surface. " +
			"Runs live by default; with --data-source local it ranks the local mirror by " +
			"engagement (bookmark*4 + comment*3 + retweet*2 + like) over an optional --window. " +
			"This is the discoverable front door to `tweets trending`.",
		Example:     "  techtwitter-pp-cli trending --limit 10 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch ranked trending tweets")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var tweets []ttTweet
			var source string

			if flags.dataSource == "local" {
				local, err := trendingLocal(ctx, ttResolveDB(dbPath), window, limit)
				if err != nil {
					return err
				}
				tweets, source = local, "local"
			} else {
				live, err := trendingLive(ctx, flags, limit)
				if err != nil {
					if flags.dataSource == "live" {
						return classifyAPIError(err, flags)
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "live fetch failed (%v); falling back to local mirror\n", err)
					local, lerr := trendingLocal(ctx, ttResolveDB(dbPath), window, limit)
					if lerr != nil {
						return classifyAPIError(err, flags)
					}
					tweets, source = local, "local"
				} else {
					tweets, source = live, "live"
				}
			}

			if ttWantsJSON(cmd, flags) {
				return ttEmitJSON(cmd, flags, tweets)
			}
			ttRenderTweetPanel(cmd, fmt.Sprintf("TRENDING (%s, %d tweets)", source, len(tweets)), tweets)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techtwitter-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum tweets to return")
	cmd.Flags().StringVar(&window, "window", "", "Local ranking window (e.g. 7d); only applies with --data-source local")
	return cmd
}

func trendingLive(ctx context.Context, flags *rootFlags, limit int) ([]ttTweet, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	params := map[string]string{}
	if limit > 0 {
		params["limit"] = strconv.Itoa(limit)
	}
	data, err := c.Get(ctx, "/api/tweets/trending", params)
	if err != nil {
		return nil, err
	}
	var env struct {
		Tweets []json.RawMessage `json:"tweets"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	out := decodeTweets(env.Tweets)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func trendingLocal(ctx context.Context, dbPath, window string, limit int) ([]ttTweet, error) {
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()
	engExpr := "(COALESCE(bookmark_count,0)*4 + COALESCE(comment_count,0)*3 + COALESCE(retweet_count,0)*2 + COALESCE(like_count,0))"
	tail := `WHERE content_type != 'article' `
	args := []any{}
	if window != "" {
		dur, err := ttParseWindow(window)
		if err != nil {
			return nil, err
		}
		tail += `AND timestamp >= ? `
		args = append(args, ttCutoff(dur))
	}
	tail += `ORDER BY ` + engExpr + ` DESC LIMIT ?`
	args = append(args, limit)
	return ttScanTweets(db.DB(), tail, args...)
}

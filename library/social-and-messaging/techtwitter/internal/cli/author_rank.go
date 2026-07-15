// Hand-authored novel command: author-rank. Ranks authors by accumulated
// engagement across the local mirror, with each author's best curated tweet.
//
// pp:data-source local

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/internal/store"
)

type ttAuthorRank struct {
	Handle          string   `json:"handle"`
	Name            string   `json:"name,omitempty"`
	TweetCount      int      `json:"tweet_count"`
	TotalEngagement int      `json:"total_engagement"`
	BestTweet       *ttTweet `json:"best_tweet,omitempty"`
}

func newNovelAuthorRankCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var window string
	var limit int

	cmd := &cobra.Command{
		Use:   "author-rank [window]",
		Short: "Rank authors by accumulated stored engagement over a window, each with their best curated tweet.",
		Long: "Aggregate the local mirror's curated tweets by author over a window (default 7d), " +
			"ranking by total engagement (bookmark*4 + comment*3 + retweet*2 + like). " +
			"Run `sync` first to populate the mirror.",
		Example:     "  techtwitter-pp-cli author-rank 7d --limit 10 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank authors from the local mirror")
				return nil
			}
			win := window
			if win == "" && len(args) == 1 {
				win = args[0]
			}
			if win == "" {
				win = "7d"
			}
			dur, err := ttParseWindow(win)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			dbPath = ttResolveDB(dbPath)
			if ttMissingMirror(cmd, flags, dbPath, "[]") {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			cutoff := ttCutoff(dur)
			rows, err := db.DB().Query(`SELECT author_handle, COALESCE(MAX(author_name),''),
				SUM(COALESCE(bookmark_count,0)*4 + COALESCE(comment_count,0)*3 + COALESCE(retweet_count,0)*2 + COALESCE(like_count,0)) AS eng,
				COUNT(*) AS cnt
				FROM tweets
				WHERE content_type != 'article' AND COALESCE(author_handle,'') != '' AND timestamp >= ?
				GROUP BY author_handle
				ORDER BY eng DESC
				LIMIT ?`, cutoff, limit)
			if err != nil {
				return fmt.Errorf("querying authors: %w", err)
			}
			defer rows.Close()

			ranked := make([]ttAuthorRank, 0)
			for rows.Next() {
				var r ttAuthorRank
				if err := rows.Scan(&r.Handle, &r.Name, &r.TotalEngagement, &r.TweetCount); err != nil {
					return fmt.Errorf("scanning author row: %w", err)
				}
				ranked = append(ranked, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			// Attach each author's best curated tweet.
			for i := range ranked {
				best, err := ttScanTweets(db.DB(),
					`WHERE author_handle = ? AND content_type != 'article'
					 ORDER BY (COALESCE(bookmark_count,0)*4 + COALESCE(comment_count,0)*3 + COALESCE(retweet_count,0)*2 + COALESCE(like_count,0)) DESC
					 LIMIT 1`, ranked[i].Handle)
				if err == nil && len(best) == 1 {
					ranked[i].BestTweet = &best[0]
				}
			}

			if len(ranked) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no authors in the last %s; run `sync` to refresh the mirror\n", dur)
			}
			if ttWantsJSON(cmd, flags) {
				return ttEmitJSON(cmd, flags, ranked)
			}
			if len(ranked) == 0 {
				return nil
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n\n", bold(fmt.Sprintf("AUTHOR RANK — last %s", dur)))
			for i, r := range ranked {
				fmt.Fprintf(w, "  %2d. @%-20s  %d eng  (%d tweets)\n", i+1, r.Handle, r.TotalEngagement, r.TweetCount)
				if r.BestTweet != nil {
					body := r.BestTweet.Text
					if body == "" {
						body = r.BestTweet.Summary
					}
					fmt.Fprintf(w, "      ↳ %s\n", truncate(body, 90))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techtwitter-pp-cli/data.db)")
	cmd.Flags().StringVar(&window, "window", "", "Lookback window (24h, 7d, 30d); also accepted as a positional")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum authors to return")
	return cmd
}

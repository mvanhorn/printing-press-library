// Hand-authored novel command: digest. Composes an offline read-list from the
// local mirror — top tweets, recent articles, and top authors for a window.
//
// pp:data-source local

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/internal/store"
)

type ttDigestAuthor struct {
	Handle          string `json:"handle"`
	TotalEngagement int    `json:"total_engagement"`
	TweetCount      int    `json:"tweet_count"`
}

type ttDigest struct {
	Window      string           `json:"window"`
	GeneratedAt string           `json:"generated_at"`
	TopTweets   []ttTweet        `json:"top_tweets"`
	Articles    []ttArticle      `json:"articles"`
	TopAuthors  []ttDigestAuthor `json:"top_authors"`
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var window string
	var limit int

	cmd := &cobra.Command{
		Use:   "digest [window]",
		Short: "Assemble a read-list from the local store for a window: top tweets, recent articles, and top authors.",
		Long: "Compose a single read-list from the offline mirror for a window (default 24h): the top " +
			"curated tweets by engagement, the most recent articles, and the top authors — each with " +
			"canonical citation URLs. Run `sync` first to populate the mirror.",
		Example:     "  techtwitter-pp-cli digest --window 24h --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compose a digest from the local mirror")
				return nil
			}
			if window == "" && len(args) == 1 {
				window = args[0]
			}
			dur, err := ttParseWindow(window)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			dbPath = ttResolveDB(dbPath)
			if ttMissingMirror(cmd, flags, dbPath, "{}") {
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
			engExpr := "(COALESCE(bookmark_count,0)*4 + COALESCE(comment_count,0)*3 + COALESCE(retweet_count,0)*2 + COALESCE(like_count,0))"

			topTweets, err := ttScanTweets(db.DB(),
				`WHERE content_type != 'article' AND timestamp >= ? ORDER BY `+engExpr+` DESC LIMIT ?`,
				cutoff, limit)
			if err != nil {
				return fmt.Errorf("querying top tweets: %w", err)
			}

			articles, err := ttScanArticles(db.DB(),
				`WHERE timestamp >= ? ORDER BY timestamp DESC LIMIT ?`, cutoff, limit)
			if err != nil {
				return fmt.Errorf("querying articles: %w", err)
			}
			// Fall back to the most recent articles if none landed in the window.
			if len(articles) == 0 {
				articles, _ = ttScanArticles(db.DB(), `ORDER BY timestamp DESC LIMIT ?`, limit)
			}

			authorRows, err := db.DB().Query(`SELECT author_handle,
				SUM(`+engExpr+`) AS eng, COUNT(*) AS cnt
				FROM tweets
				WHERE content_type != 'article' AND COALESCE(author_handle,'') != '' AND timestamp >= ?
				GROUP BY author_handle ORDER BY eng DESC LIMIT ?`, cutoff, limit)
			if err != nil {
				return fmt.Errorf("querying top authors: %w", err)
			}
			defer authorRows.Close()
			authors := make([]ttDigestAuthor, 0)
			for authorRows.Next() {
				var a ttDigestAuthor
				if err := authorRows.Scan(&a.Handle, &a.TotalEngagement, &a.TweetCount); err != nil {
					return err
				}
				authors = append(authors, a)
			}
			if err := authorRows.Err(); err != nil {
				return fmt.Errorf("iterating author rows: %w", err)
			}

			digest := ttDigest{
				Window:      dur.String(),
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				TopTweets:   topTweets,
				Articles:    articles,
				TopAuthors:  authors,
			}

			if len(topTweets) == 0 && len(articles) == 0 && len(authors) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no curated data in the last %s; run `sync` to refresh the mirror\n", dur)
			}
			if ttWantsJSON(cmd, flags) {
				return ttEmitJSON(cmd, flags, digest)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n\n", bold(fmt.Sprintf("TECH TWITTER DIGEST — last %s", dur)))
			ttRenderTweetPanel(cmd, fmt.Sprintf("Top tweets (%d)", len(topTweets)), topTweets)
			fmt.Fprintf(w, "%s\n", bold(fmt.Sprintf("Articles (%d)", len(articles))))
			for _, a := range articles {
				fmt.Fprintf(w, "  • %s\n    %s\n", truncate(a.Title, 90), a.URL)
			}
			fmt.Fprintf(w, "\n%s\n", bold(fmt.Sprintf("Top authors (%d)", len(authors))))
			for i, a := range authors {
				fmt.Fprintf(w, "  %2d. @%-20s  %d eng (%d tweets)\n", i+1, a.Handle, a.TotalEngagement, a.TweetCount)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techtwitter-pp-cli/data.db)")
	cmd.Flags().StringVar(&window, "window", "", "Lookback window (24h, 48h, 7d, 30d); also accepted as a positional")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum items per section")
	return cmd
}

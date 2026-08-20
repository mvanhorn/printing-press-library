// Hand-authored novel command: since. Shows tweets curated/newly-hot within a
// window, read from the offline local mirror (no network).
//
// pp:data-source local

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/internal/store"
)

func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "since [window]",
		Short: "See only the tweets curated or newly hot since your last sync, instead of re-reading the whole stream.",
		Long: "Show curated tweets whose timestamp falls within the given window (default 24h), " +
			"read entirely from the local SQLite mirror. Run `sync` first to populate it. " +
			"Window accepts 24h, 48h, 7d, 30d, etc.",
		Example:     "  techtwitter-pp-cli since 24h --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query local mirror for tweets within window")
				return nil
			}
			window := ""
			if len(args) == 1 {
				window = args[0]
			}
			dur, err := ttParseWindow(window)
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
			tweets, err := ttScanTweets(db.DB(),
				`WHERE content_type != 'article' AND timestamp >= ? ORDER BY timestamp DESC LIMIT ?`,
				cutoff, limit)
			if err != nil {
				return fmt.Errorf("querying tweets: %w", err)
			}

			if ttWantsJSON(cmd, flags) {
				return ttEmitJSON(cmd, flags, tweets)
			}
			if len(tweets) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no curated tweets in the last %s; run `sync` to refresh the mirror\n", dur)
				return nil
			}
			ttRenderTweetPanel(cmd, fmt.Sprintf("SINCE %s — %d tweets", dur, len(tweets)), tweets)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techtwitter-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum tweets to return")
	return cmd
}

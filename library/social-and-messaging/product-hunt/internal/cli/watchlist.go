// Copyright 2026 actionsslave. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/product-hunt/internal/store"
	"github.com/spf13/cobra"
)

func newWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Pin posts to a watchlist and batch-refresh stats with vote/comment deltas",
		Long: `Track your launches or competitors' launches. Pin posts by slug, then run
'watchlist refresh' to see current vote and comment counts alongside the
delta since you last checked.`,
	}
	cmd.AddCommand(newWatchlistAddCmd(flags))
	cmd.AddCommand(newWatchlistRemoveCmd(flags))
	cmd.AddCommand(newWatchlistListCmd(flags))
	cmd.AddCommand(newWatchlistRefreshCmd(flags))
	return cmd
}

func newWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:     "add <slug>",
		Short:   "Add a post to your watchlist",
		Example: "  product-hunt-pp-cli watchlist add my-product-slug",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("product-hunt-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureWatchlistTable(db); err != nil {
				return err
			}

			phc, err := flags.newPHClient()
			if err != nil {
				return err
			}

			slug := strings.TrimSpace(args[0])
			post, err := phc.GetPost(cmd.Context(), slug)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			now := time.Now().UTC().Format(time.RFC3339)
			_, err = db.DB().Exec(`INSERT INTO watchlist(slug, name, votes_count, comments_count, last_seen, added_at)
				VALUES(?, ?, ?, ?, ?, ?)
				ON CONFLICT(slug) DO UPDATE SET
					name = excluded.name,
					votes_count = excluded.votes_count,
					comments_count = excluded.comments_count,
					last_seen = excluded.last_seen`,
				post.Slug, post.Name, post.VotesCount, post.CommentsCount, now, now)
			if err != nil {
				return fmt.Errorf("saving to watchlist: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added %q to watchlist (%d votes, %d comments).\n",
				post.Name, post.VotesCount, post.CommentsCount)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchlistRemoveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:     "remove <slug>",
		Short:   "Remove a post from your watchlist",
		Example: "  product-hunt-pp-cli watchlist remove my-product-slug",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("product-hunt-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureWatchlistTable(db); err != nil {
				return err
			}

			slug := strings.TrimSpace(args[0])
			_, err = db.DB().Exec(`DELETE FROM watchlist WHERE slug = ?`, slug)
			if err != nil {
				return fmt.Errorf("removing from watchlist: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %q from watchlist.\n", slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchlistListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "Show all posts in your watchlist",
		Example: "  product-hunt-pp-cli watchlist list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("product-hunt-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureWatchlistTable(db); err != nil {
				return err
			}

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT slug, name, votes_count, comments_count, last_seen, added_at FROM watchlist ORDER BY added_at DESC`)
			if err != nil {
				return fmt.Errorf("querying watchlist: %w", err)
			}
			defer rows.Close()

			var items []map[string]any
			for rows.Next() {
				var slug, name, lastSeen, addedAt string
				var votes, comments int
				if err := rows.Scan(&slug, &name, &votes, &comments, &lastSeen, &addedAt); err != nil {
					continue
				}
				items = append(items, map[string]any{
					"slug":          slug,
					"name":          name,
					"votesCount":    votes,
					"commentsCount": comments,
					"lastSeen":      lastSeen,
					"addedAt":       addedAt,
				})
			}

			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist is empty. Run 'watchlist add <slug>' to add posts.")
				return nil
			}

			data, err := json.Marshal(items)
			if err != nil {
				return err
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type watchlistRefreshItem struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	VotesCount    int    `json:"votesCount"`
	CommentsCount int    `json:"commentsCount"`
	VotesDelta    int    `json:"votesDelta"`
	CommentsDelta int    `json:"commentsDelta"`
	LastSeen      string `json:"lastSeen"`
	URL           string `json:"url,omitempty"`
}

func newWatchlistRefreshCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Fetch live stats for all watchlisted posts and show deltas",
		Long: `Calls the live API for every post on your watchlist and shows the vote and
comment count change since your last refresh. Great for launch-day monitoring
of multiple products at once.`,
		Example: strings.Trim(`
  product-hunt-pp-cli watchlist refresh
  product-hunt-pp-cli watchlist refresh --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("product-hunt-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureWatchlistTable(db); err != nil {
				return err
			}

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT slug, name, votes_count, comments_count FROM watchlist ORDER BY added_at DESC`)
			if err != nil {
				return fmt.Errorf("querying watchlist: %w", err)
			}

			type wItem struct {
				slug     string
				name     string
				prevVotes int
				prevComments int
			}
			var watchItems []wItem
			for rows.Next() {
				var w wItem
				if err := rows.Scan(&w.slug, &w.name, &w.prevVotes, &w.prevComments); err != nil {
					continue
				}
				watchItems = append(watchItems, w)
			}
			rows.Close()

			if len(watchItems) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist is empty. Run 'watchlist add <slug>' first.")
				return nil
			}

			phc, err := flags.newPHClient()
			if err != nil {
				return err
			}

			now := time.Now().UTC().Format(time.RFC3339)
			var results []watchlistRefreshItem
			for _, w := range watchItems {
				post, err := phc.GetPost(cmd.Context(), w.slug)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", w.slug, err)
					continue
				}
				results = append(results, watchlistRefreshItem{
					Slug:          post.Slug,
					Name:          post.Name,
					VotesCount:    post.VotesCount,
					CommentsCount: post.CommentsCount,
					VotesDelta:    post.VotesCount - w.prevVotes,
					CommentsDelta: post.CommentsCount - w.prevComments,
					LastSeen:      now,
					URL:           post.URL,
				})
				_, _ = db.DB().Exec(`UPDATE watchlist SET votes_count=?, comments_count=?, last_seen=? WHERE slug=?`,
					post.VotesCount, post.CommentsCount, now, post.Slug)
			}

			data, err := json.Marshal(results)
			if err != nil {
				return err
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func ensureWatchlistTable(db *store.Store) error {
	_, err := db.DB().Exec(`CREATE TABLE IF NOT EXISTS watchlist (
		slug TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		votes_count INTEGER NOT NULL DEFAULT 0,
		comments_count INTEGER NOT NULL DEFAULT 0,
		last_seen TEXT,
		added_at TEXT NOT NULL
	)`)
	return err
}

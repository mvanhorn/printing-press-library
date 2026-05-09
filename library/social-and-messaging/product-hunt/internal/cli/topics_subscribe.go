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

// newTopicsSubscribeCmd, newTopicsUnsubscribeCmd, newTopicsInboxCmd are
// subcommands that let users subscribe to topics and receive a cursor-based
// inbox showing only new posts since their last check.

func newTopicsSubscribeCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "subscribe <topic-slug>",
		Short: "Subscribe to a topic to track new posts in your inbox",
		Example: strings.Trim(`
  product-hunt-pp-cli topics subscribe ai
  product-hunt-pp-cli topics subscribe developer-tools`, "\n"),
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

			if err := ensureSubscriptionsTable(db); err != nil {
				return err
			}

			slug := strings.ToLower(strings.TrimSpace(args[0]))
			now := time.Now().UTC().Format(time.RFC3339)
			_, err = db.DB().Exec(`INSERT INTO topic_subscriptions(slug, last_cursor, subscribed_at)
				VALUES(?, '', ?)
				ON CONFLICT(slug) DO NOTHING`, slug, now)
			if err != nil {
				return fmt.Errorf("saving subscription: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Subscribed to topic %q. Run 'topics inbox' to see new posts.\n", slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newTopicsUnsubscribeCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:     "unsubscribe <topic-slug>",
		Short:   "Unsubscribe from a topic",
		Example: "  product-hunt-pp-cli topics unsubscribe ai",
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

			if err := ensureSubscriptionsTable(db); err != nil {
				return err
			}

			slug := strings.ToLower(strings.TrimSpace(args[0]))
			_, err = db.DB().Exec(`DELETE FROM topic_subscriptions WHERE slug = ?`, slug)
			if err != nil {
				return fmt.Errorf("removing subscription: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Unsubscribed from topic %q.\n", slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newTopicsInboxCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Show new posts since your last check for all subscribed topics",
		Long: `Fetches new posts for each subscribed topic using a cursor, so you never
see the same post twice. Perfect for staying on top of launch activity
in your favourite categories without missing anything.

Subscribe first: topics subscribe <slug>
Then run: topics inbox`,
		Example: strings.Trim(`
  product-hunt-pp-cli topics inbox
  product-hunt-pp-cli topics inbox --json --select name,tagline,votesCount`, "\n"),
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

			if err := ensureSubscriptionsTable(db); err != nil {
				return err
			}

			// Load subscriptions
			subRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT slug, last_cursor FROM topic_subscriptions`)
			if err != nil {
				return fmt.Errorf("loading subscriptions: %w", err)
			}
			type sub struct {
				slug   string
				cursor string
			}
			var subs []sub
			for subRows.Next() {
				var s sub
				if err := subRows.Scan(&s.slug, &s.cursor); err != nil {
					continue
				}
				subs = append(subs, s)
			}
			subRows.Close()

			if len(subs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No subscriptions yet. Run 'topics subscribe <slug>' first.")
				return nil
			}

			phc, err := flags.newPHClient()
			if err != nil {
				return err
			}

			type inboxItem struct {
				Topic         string `json:"topic"`
				Name          string `json:"name"`
				Slug          string `json:"slug"`
				Tagline       string `json:"tagline"`
				VotesCount    int    `json:"votesCount"`
				CommentsCount int    `json:"commentsCount"`
				FeaturedAt    string `json:"featuredAt,omitempty"`
				URL           string `json:"url,omitempty"`
			}

			var allItems []inboxItem
			for _, s := range subs {
				conn, err := phc.GetPosts(cmd.Context(), limit, s.cursor, s.slug, "NEWEST", false, "", "")
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: fetching %s: %v\n", s.slug, err)
					continue
				}

				newCursor := conn.PageInfo.EndCursor
				for _, e := range conn.Edges {
					p := e.Node
					allItems = append(allItems, inboxItem{
						Topic:         s.slug,
						Name:          p.Name,
						Slug:          p.Slug,
						Tagline:       p.Tagline,
						VotesCount:    p.VotesCount,
						CommentsCount: p.CommentsCount,
						FeaturedAt:    p.FeaturedAt,
						URL:           p.URL,
					})
				}

				// Advance cursor
				if newCursor != "" {
					_, _ = db.DB().Exec(`UPDATE topic_subscriptions SET last_cursor = ?, last_checked = ? WHERE slug = ?`,
						newCursor, time.Now().UTC().Format(time.RFC3339), s.slug)
				}
			}

			if len(allItems) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No new posts since your last check.")
				return nil
			}

			data, err := json.Marshal(allItems)
			if err != nil {
				return err
			}

			prov := DataProvenance{Source: "live"}
			printProvenance(cmd, len(allItems), prov)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, err := wrapWithProvenance(filtered, prov)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					return printAutoTable(cmd.OutOrStdout(), items)
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum new posts per topic")
	return cmd
}

func ensureSubscriptionsTable(db *store.Store) error {
	_, err := db.DB().Exec(`CREATE TABLE IF NOT EXISTS topic_subscriptions (
		slug TEXT PRIMARY KEY,
		last_cursor TEXT NOT NULL DEFAULT '',
		last_checked TEXT,
		subscribed_at TEXT NOT NULL
	)`)
	return err
}

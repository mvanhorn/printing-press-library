package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/atom"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"
	"github.com/spf13/cobra"
)

func newTodayCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var live bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "today",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "List the current featured launches on Product Hunt",
		Long: `Show the top entries from the public Product Hunt Atom feed.

By default reads from the local store (if a sync has been run) and falls
back to a live /feed fetch. Use --live to always bypass the store.

Each entry is a ranked post with id, slug, title, tagline, author, canonical
discussion URL, external URL, and published/updated timestamps. Rank 1 is
the topmost entry in the feed at fetch time.`,
		Example: `  # Top 10 right now
  producthunt-pp-cli today --limit 10

  # Agent-friendly narrow payload
  producthunt-pp-cli today --limit 5 --agent --select 'slug,title,tagline,author'

  # Always fetch from the live feed (bypass store)
  producthunt-pp-cli today --live`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				limit = 10
			}
			autoWarm(flags, dbPath)

			// Resolve data source: --live forces network; otherwise prefer
			// store with live fallback when store is empty.
			if !live {
				db, err := openStore(dbPath)
				if err == nil {
					defer db.Close()
					latest, err := db.LatestSnapshot()
					if err == nil && latest != nil {
						posts, err := db.PostsInSnapshot(latest.SnapshotID)
						if err == nil && len(posts) > 0 {
							if limit > len(posts) {
								limit = len(posts)
							}
							payloads := make([]postPayload, limit)
							for i := 0; i < limit; i++ {
								pp := postPayloadOf(posts[i])
								pp.Rank = i + 1
								payloads[i] = pp
							}
							buf, _ := json.Marshal(payloads)
							return printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
						}
					}
				}
			}

			// Live fetch path
			body, err := fetchFeedBody(flags.timeout)
			if err != nil {
				return apiErr(err)
			}
			feed, err := atom.Parse(body)
			if err != nil {
				return apiErr(fmt.Errorf("parse feed: %w", err))
			}
			if limit > len(feed.Entries) {
				limit = len(feed.Entries)
			}
			payloads := make([]postPayload, limit)
			for i := 0; i < limit; i++ {
				payloads[i] = atomEntryToPayload(feed.Entries[i], i+1)
			}
			buf, _ := json.Marshal(payloads)
			return printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Max entries to return (1-50)")
	cmd.Flags().BoolVar(&live, "live", false, "Always fetch /feed live, bypass the local store")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite store (default: ~/.local/share/producthunt-pp-cli/data.db)")
	return cmd
}

// atomEntryToPayload converts a live-fetched atom.Entry into the stable JSON
// payload shape. Only called on the live fetch path — stored posts go through
// postPayloadOf directly.
func atomEntryToPayload(e atom.Entry, rank int) postPayload {
	pp := postPayload{
		ID:            e.PostID,
		Slug:          e.Slug,
		Title:         e.Title,
		Tagline:       e.Tagline,
		Author:        e.Author,
		DiscussionURL: e.DiscussionURL,
		ExternalURL:   e.ExternalURL,
		Rank:          rank,
	}
	if !e.Published.IsZero() {
		pp.Published = e.Published.UTC().Format(time.RFC3339)
	}
	if !e.Updated.IsZero() {
		pp.Updated = e.Updated.UTC().Format(time.RFC3339)
	}
	return pp
}

func newRecentCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "recent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Live-fetch /feed (bypass store) and return the newest entries",
		Long: `Shortcut for 'today --live'. Always fetches /feed from the network and does
not read from the local store. Useful when you want a fresh view without
sync'ing.`,
		Example: `  producthunt-pp-cli recent --limit 15 --json --select 'slug,title,published'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = dbPath
			body, err := fetchFeedBody(flags.timeout)
			if err != nil {
				return apiErr(err)
			}
			feed, err := atom.Parse(body)
			if err != nil {
				return apiErr(fmt.Errorf("parse feed: %w", err))
			}
			if limit <= 0 {
				limit = 10
			}
			if limit > len(feed.Entries) {
				limit = len(feed.Entries)
			}
			payloads := make([]postPayload, limit)
			for i := 0; i < limit; i++ {
				payloads[i] = atomEntryToPayload(feed.Entries[i], i+1)
			}
			_ = store.Post{}
			buf, _ := json.Marshal(payloads)
			return printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Max entries to return")
	return cmd
}

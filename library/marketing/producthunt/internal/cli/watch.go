package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/atom"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"
	"github.com/spf13/cobra"
)

// watchPayload is the diff result: entries in the current /feed that were
// NOT in the previous snapshot (by PostID). Idempotent across back-to-back
// runs at the same rate.
type watchPayload struct {
	PreviousSnapshotID int64         `json:"previous_snapshot_id,omitempty"`
	PreviousTakenAt    string        `json:"previous_taken_at,omitempty"`
	CurrentTakenAt     string        `json:"current_taken_at"`
	NewCount           int           `json:"new_count"`
	NewEntries         []postPayload `json:"new_entries"`
	Elapsed            float64       `json:"elapsed_secs"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var noWrite bool

	cmd := &cobra.Command{
		Use:   "watch",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show entries new since the last sync (and optionally record a new snapshot)",
		Long: `Diff the live /feed against the most recent snapshot in the store. Returns
entries whose PostID isn't in that snapshot — the "new" set.

By default, also records a new snapshot so the next run diffs against fresh
data. Use --no-write to compute the diff without updating the store (handy
for previewing what a schedule run would surface).

Idempotent when the feed hasn't changed; exit code 0 with new_count=0 is
the normal no-op.`,
		Example: `  # Diff-on-last-sync, record new snapshot
  producthunt-pp-cli watch --agent

  # Preview without writing
  producthunt-pp-cli watch --no-write --json

  # Agent-narrow: just new slugs
  producthunt-pp-cli watch --agent --select 'new_entries.slug,new_entries.title'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			db, err := openStore(dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			prev, err := db.LatestSnapshot()
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return apiErr(err)
			}
			var prevIDs map[int64]struct{}
			if prev != nil {
				prevIDs = map[int64]struct{}{}
				posts, err := db.PostsInSnapshot(prev.SnapshotID)
				if err != nil {
					return apiErr(err)
				}
				for _, p := range posts {
					prevIDs[p.PostID] = struct{}{}
				}
			}

			body, err := fetchFeedBody(flags.timeout)
			if err != nil {
				return apiErr(err)
			}
			feed, err := atom.Parse(body)
			if err != nil {
				return apiErr(fmt.Errorf("parse feed: %w", err))
			}

			var diffEntries []atom.Entry
			for _, e := range feed.Entries {
				if prevIDs == nil {
					diffEntries = append(diffEntries, e)
					continue
				}
				if _, seen := prevIDs[e.PostID]; !seen {
					diffEntries = append(diffEntries, e)
				}
			}

			out := watchPayload{
				CurrentTakenAt: time.Now().UTC().Format(time.RFC3339),
				NewCount:       len(diffEntries),
			}
			if prev != nil {
				out.PreviousSnapshotID = prev.SnapshotID
				out.PreviousTakenAt = prev.TakenAt.UTC().Format(time.RFC3339)
			}
			payloads := make([]postPayload, len(diffEntries))
			for i, e := range diffEntries {
				payloads[i] = atomEntryToPayload(e, i+1)
			}
			out.NewEntries = payloads
			out.Elapsed = time.Since(start).Seconds()

			if !noWrite {
				tx, err := db.DB().Begin()
				if err != nil {
					return apiErr(fmt.Errorf("begin tx: %w", err))
				}
				commit := false
				defer func() {
					if !commit {
						_ = tx.Rollback()
					}
				}()

				snapID, err := store.RecordSnapshot(tx, time.Now(), len(feed.Entries), "feed")
				if err != nil {
					return apiErr(err)
				}
				for i, e := range feed.Entries {
					if e.Slug == "" {
						continue
					}
					p := store.Post{
						PostID:        e.PostID,
						Slug:          e.Slug,
						Title:         e.Title,
						Tagline:       e.Tagline,
						Author:        e.Author,
						DiscussionURL: e.DiscussionURL,
						ExternalURL:   e.ExternalURL,
						PublishedAt:   e.Published,
						UpdatedAt:     e.Updated,
					}
					if err := store.UpsertPost(tx, p); err != nil {
						return apiErr(err)
					}
					if err := store.RecordSnapshotEntry(tx, snapID, e.PostID, i+1, e.ExternalURL); err != nil {
						return apiErr(err)
					}
				}
				if err := tx.Commit(); err != nil {
					return apiErr(err)
				}
				commit = true
			}

			buf, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite store")
	cmd.Flags().BoolVar(&noWrite, "no-write", false, "Compute the diff but do not record a new snapshot")
	return cmd
}

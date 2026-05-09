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

type momentumSnapshot struct {
	Slug          string `json:"slug"`
	VotesCount    int    `json:"votesCount"`
	CommentsCount int    `json:"commentsCount"`
	LastSeen      string `json:"lastSeen"`
}

type momentumResult struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	VotesCount    int    `json:"votesCount"`
	CommentsCount int    `json:"commentsCount"`
	VotesDelta    int    `json:"votesDelta"`
	CommentsDelta int    `json:"commentsDelta"`
	LastChecked   string `json:"lastChecked,omitempty"`
	URL           string `json:"url,omitempty"`
}

func newPostsMomentumCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "momentum <slug>",
		Short: "Show live vote and comment deltas since your last check",
		Long: `Fetches the current vote and comment counts for a post from the live API,
compares with the last-seen snapshot stored locally, and shows the delta.

First run for a post: saves the current counts as the baseline.
Subsequent runs: shows how much the post has grown since you last checked.

Perfect for monitoring launch-day momentum without refreshing the browser.`,
		Example: strings.Trim(`
  product-hunt-pp-cli posts momentum my-product-slug
  product-hunt-pp-cli posts momentum my-product-slug --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
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

			if err := ensureMomentumTable(db); err != nil {
				return err
			}

			phc, err := flags.newPHClient()
			if err != nil {
				return err
			}

			// pp:client-call — live API call via phgraphql client
			post, err := phc.GetPost(cmd.Context(), args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Load previous snapshot
			prev, _ := loadMomentumSnapshot(db, post.Slug)

			now := time.Now().UTC().Format(time.RFC3339)
			result := momentumResult{
				Slug:          post.Slug,
				Name:          post.Name,
				VotesCount:    post.VotesCount,
				CommentsCount: post.CommentsCount,
				URL:           post.URL,
				LastChecked:   now,
			}
			if prev != nil {
				result.VotesDelta = post.VotesCount - prev.VotesCount
				result.CommentsDelta = post.CommentsCount - prev.CommentsCount
			}

			// Save new snapshot
			_ = saveMomentumSnapshot(db, momentumSnapshot{
				Slug:          post.Slug,
				VotesCount:    post.VotesCount,
				CommentsCount: post.CommentsCount,
				LastSeen:      now,
			})

			data, err := json.Marshal(result)
			if err != nil {
				return err
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func ensureMomentumTable(db *store.Store) error {
	_, err := db.DB().Exec(`CREATE TABLE IF NOT EXISTS momentum_snapshots (
		slug TEXT PRIMARY KEY,
		votes_count INTEGER NOT NULL DEFAULT 0,
		comments_count INTEGER NOT NULL DEFAULT 0,
		last_seen TEXT NOT NULL
	)`)
	return err
}

func loadMomentumSnapshot(db *store.Store, slug string) (*momentumSnapshot, error) {
	row := db.DB().QueryRow(`SELECT slug, votes_count, comments_count, last_seen FROM momentum_snapshots WHERE slug = ?`, slug)
	var s momentumSnapshot
	if err := row.Scan(&s.Slug, &s.VotesCount, &s.CommentsCount, &s.LastSeen); err != nil {
		return nil, err
	}
	return &s, nil
}

func saveMomentumSnapshot(db *store.Store, s momentumSnapshot) error {
	_, err := db.DB().Exec(`INSERT INTO momentum_snapshots(slug, votes_count, comments_count, last_seen)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			votes_count = excluded.votes_count,
			comments_count = excluded.comments_count,
			last_seen = excluded.last_seen`,
		s.Slug, s.VotesCount, s.CommentsCount, s.LastSeen)
	return err
}

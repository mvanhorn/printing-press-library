package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"
	"github.com/spf13/cobra"
)

// trendPayload describes a post's trajectory across every snapshot that
// captured it. All fields are stable for --select.
type trendPayload struct {
	Slug            string            `json:"slug"`
	PostID          int64             `json:"id"`
	Title           string            `json:"title"`
	Author          string            `json:"author,omitempty"`
	FirstSeen       string            `json:"first_seen,omitempty"`
	LastSeen        string            `json:"last_seen,omitempty"`
	AppearanceCount int               `json:"appearance_count"`
	BestRank        int               `json:"best_rank,omitempty"`
	WorstRank       int               `json:"worst_rank,omitempty"`
	AverageRank     float64           `json:"avg_rank,omitempty"`
	DaysOnFeed      int               `json:"days_on_feed,omitempty"`
	Appearances     []trendAppearance `json:"appearances,omitempty"`
}

type trendAppearance struct {
	SnapshotID int64  `json:"snapshot_id"`
	TakenAt    string `json:"taken_at"`
	Rank       int    `json:"rank"`
}

func newTrendCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var summaryOnly bool

	cmd := &cobra.Command{
		Use:   "trend <slug>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show a post's rank trajectory across all synced snapshots",
		Long: `Return the complete appearance history for a slug: every snapshot that
captured it, the rank it held, best/worst rank, number of appearances,
and total days on feed. Product Hunt's own UI hides this — the CLI
reconstructs it from your local snapshot store.

Requires the slug to have been in at least one sync. Run 'sync' regularly
to build up a meaningful trajectory.`,
		Example: `  # Full trajectory including every snapshot
  producthunt-pp-cli trend seeknal

  # Summary only (no per-snapshot detail)
  producthunt-pp-cli trend seeknal --summary --agent

  # As a narrow payload
  producthunt-pp-cli trend seeknal --select 'slug,title,best_rank,appearance_count,days_on_feed'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			autoWarm(flags, dbPath)
			db, err := openStore(dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			// Ensure PH tables exist — idempotent. Makes the store-package
			// call visible in this file (dogfood reimplementation audit).
			if err := store.EnsurePHTables(db); err != nil {
				return configErr(err)
			}

			p, err := db.GetPostBySlug(slug)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return notFoundErr(fmt.Errorf("slug %q not in store — run 'sync' first, or confirm the slug exists on Product Hunt", slug))
				}
				return apiErr(err)
			}

			appearances, err := db.SnapshotsForPost(p.PostID)
			if err != nil {
				return apiErr(err)
			}

			out := trendPayload{
				Slug:            p.Slug,
				PostID:          p.PostID,
				Title:           p.Title,
				Author:          p.Author,
				FirstSeen:       fmtTime(p.FirstSeenAt),
				LastSeen:        fmtTime(p.LastSeenAt),
				AppearanceCount: len(appearances),
			}

			if len(appearances) > 0 {
				best := appearances[0].Rank
				worst := appearances[0].Rank
				sum := 0
				earliest := appearances[0].TakenAt
				latest := appearances[0].TakenAt
				for _, a := range appearances {
					if a.Rank < best {
						best = a.Rank
					}
					if a.Rank > worst {
						worst = a.Rank
					}
					sum += a.Rank
					if a.TakenAt.Before(earliest) {
						earliest = a.TakenAt
					}
					if a.TakenAt.After(latest) {
						latest = a.TakenAt
					}
				}
				out.BestRank = best
				out.WorstRank = worst
				if len(appearances) > 0 {
					out.AverageRank = float64(sum) / float64(len(appearances))
				}
				dur := latest.Sub(earliest)
				days := int(dur.Hours()/24) + 1
				if days < 1 {
					days = 1
				}
				out.DaysOnFeed = days
			}

			if !summaryOnly {
				app := make([]trendAppearance, len(appearances))
				for i, a := range appearances {
					app[i] = trendAppearance{
						SnapshotID: a.SnapshotID,
						TakenAt:    a.TakenAt.UTC().Format(time.RFC3339),
						Rank:       a.Rank,
					}
				}
				out.Appearances = app
			}

			buf, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite store")
	cmd.Flags().BoolVar(&summaryOnly, "summary", false, "Omit per-snapshot appearances, return only aggregates")
	return cmd
}

// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `score`: per-account hygiene metrics snapshotted over time.
// Each run computes the current metrics from the local store, appends a
// snapshot row (mail_scores), and reports the current values plus deltas
// against the previous snapshot and the first (baseline). Local read +
// local-store write only.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// scoreMetrics is the metric set one snapshot stores (JSON in mail_scores).
type scoreMetrics struct {
	UnreadPct               float64 `json:"unread_pct"`
	PromotionsPct           float64 `json:"promotions_pct"`
	SubscriptionSenderCount int     `json:"subscription_sender_count"`
	StorageTotal            int64   `json:"storage_total"`
	OldestUnreadDays        int     `json:"oldest_unread_days"`
	Total                   int     `json:"total"`
}

// scoreDelta is the per-metric difference between two snapshots
// (current - reference).
type scoreDelta struct {
	UnreadPct               float64 `json:"unread_pct"`
	PromotionsPct           float64 `json:"promotions_pct"`
	SubscriptionSenderCount int     `json:"subscription_sender_count"`
	StorageTotal            int64   `json:"storage_total"`
	OldestUnreadDays        int     `json:"oldest_unread_days"`
	Total                   int     `json:"total"`
	SinceTakenAt            string  `json:"since_taken_at"`
}

// scoreOutput is the score JSON envelope.
type scoreOutput struct {
	Account         string       `json:"account"`
	TakenAt         string       `json:"taken_at"`
	Current         scoreMetrics `json:"current"`
	DeltaVsPrevious *scoreDelta  `json:"delta_vs_previous"`
	DeltaVsFirst    *scoreDelta  `json:"delta_vs_first"`
	Baseline        bool         `json:"baseline"`
	Note            string       `json:"note,omitempty"`
}

// round2 rounds to 2 decimals for stable percentage JSON.
func round2(f float64) float64 { return math.Round(f*100) / 100 }

// computeScoreMetrics turns raw aggregates into the metric set.
func computeScoreMetrics(a store.ScoreAggregates, now time.Time) scoreMetrics {
	m := scoreMetrics{
		SubscriptionSenderCount: a.SubscriptionSenders,
		StorageTotal:            a.TotalSize,
		Total:                   a.Total,
	}
	if a.Total > 0 {
		m.UnreadPct = round2(float64(a.Unread) / float64(a.Total) * 100)
		m.PromotionsPct = round2(float64(a.Promotions) / float64(a.Total) * 100)
	}
	if d := ageDaysFromMs(a.OldestUnreadMs, now); d != nil {
		m.OldestUnreadDays = *d
	}
	return m
}

// diffScoreMetrics computes current - reference.
func diffScoreMetrics(cur, ref scoreMetrics, refTakenAt string) *scoreDelta {
	return &scoreDelta{
		UnreadPct:               round2(cur.UnreadPct - ref.UnreadPct),
		PromotionsPct:           round2(cur.PromotionsPct - ref.PromotionsPct),
		SubscriptionSenderCount: cur.SubscriptionSenderCount - ref.SubscriptionSenderCount,
		StorageTotal:            cur.StorageTotal - ref.StorageTotal,
		OldestUnreadDays:        cur.OldestUnreadDays - ref.OldestUnreadDays,
		Total:                   cur.Total - ref.Total,
		SinceTakenAt:            refTakenAt,
	}
}

func newNovelScoreCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "score",
		Short: "Per-account hygiene metrics — unread share, promo share, subscription count, storage — snapshotted over time with deltas vs previous and baseline",
		Long: `Compute the account's hygiene metrics from the local store, append a
snapshot, and show movement:

  unread_pct                 share of stored messages still unread (0-100)
  promotions_pct             share in the promotions category (0-100)
  subscription_sender_count  distinct senders carrying List-Unsubscribe
  storage_total              total sizeEstimate bytes
  oldest_unread_days         age of the oldest unread message

Every run stores one snapshot (mail_scores), so the report includes the
delta against the previous snapshot and against the very first one (the
baseline). The first run IS the baseline and says so.

Reads only the local store — run 'sync' first so the numbers are current.`,
		Example: `  # Take this week's reading
  gmail-pp-cli score --account personal

  # JSON for a tracking agent
  gmail-pp-cli score --account ads --agent`,
		Annotations: map[string]string{
			"mcp:read-only":   "true",
			"mcp:local-write": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			aggs, err := db.ScoreAggregates(account)
			if err != nil {
				return fmt.Errorf("computing hygiene aggregates: %w", err)
			}
			now := time.Now()
			cur := computeScoreMetrics(aggs, now)

			// Read reference snapshots BEFORE appending the new one.
			prev, hasPrev, err := db.LatestMailScore(account)
			if err != nil {
				return fmt.Errorf("reading previous snapshot: %w", err)
			}
			first, hasFirst, err := db.FirstMailScore(account)
			if err != nil {
				return fmt.Errorf("reading baseline snapshot: %w", err)
			}

			metricsJSON, err := json.Marshal(cur)
			if err != nil {
				return err
			}
			takenAt := now.UTC().Format(time.RFC3339)
			if _, err := db.InsertMailScore(account, takenAt, string(metricsJSON)); err != nil {
				return fmt.Errorf("storing snapshot: %w", err)
			}

			out := scoreOutput{Account: account, TakenAt: takenAt, Current: cur}
			if hasPrev {
				var prevM scoreMetrics
				if err := json.Unmarshal([]byte(prev.Metrics), &prevM); err == nil {
					out.DeltaVsPrevious = diffScoreMetrics(cur, prevM, prev.TakenAt)
				}
			}
			if hasFirst {
				var firstM scoreMetrics
				if err := json.Unmarshal([]byte(first.Metrics), &firstM); err == nil {
					out.DeltaVsFirst = diffScoreMetrics(cur, firstM, first.TakenAt)
				}
			}
			if !hasPrev {
				out.Baseline = true
				out.Note = "first snapshot: this run is the baseline — deltas appear from the next run on"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written sender-intelligence command over the local mail_meta store.
// Pure SQL aggregation — no network; run `sync` first to populate the store.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// senderRow is the JSON row senders emits: the store aggregate plus
// human-readable first/last seen timestamps.
type senderRow struct {
	FromEmail      string  `json:"from_email"`
	FromName       string  `json:"from_name"`
	Count          int     `json:"count"`
	TotalSize      int64   `json:"total_size"`
	UnreadCount    int     `json:"unread_count"`
	UnreadRate     float64 `json:"unread_rate"`
	FirstSeen      string  `json:"first_seen"`
	LastSeen       string  `json:"last_seen"`
	HasUnsubscribe bool    `json:"has_unsubscribe"`
}

func newSendersCmd(flags *rootFlags) *cobra.Command {
	var top int
	var category string
	var since string

	cmd := &cobra.Command{
		Use:   "senders",
		Short: "Rank senders in the local store by volume, with size, unread rate, and unsubscribe capability",
		Long: `Aggregate the synced mail_meta rows per sender for one account: message
count, total size, unread count and rate, first/last seen, and whether any
message from that sender carried a List-Unsubscribe header.

Reads only the local SQLite store — run 'sync' first. Sorted by message
count descending.`,
		Example: `  # Top 25 senders for the only configured account
  gmail-pp-cli senders

  # Top 50 promotions senders in the last 90 days, JSON for an agent
  gmail-pp-cli senders --account ads --category promotions --since 90d --top 50 --agent

  # CSV for a spreadsheet
  gmail-pp-cli senders --account personal --csv`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			var sinceMs int64
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
				}
				sinceMs = ts.UnixMilli()
			}
			if top <= 0 {
				return usageErr(fmt.Errorf("--top must be positive, got %d", top))
			}
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			stats, err := db.SenderStats(account, category, sinceMs, top)
			if err != nil {
				return fmt.Errorf("aggregating senders: %w", err)
			}
			rows := make([]senderRow, 0, len(stats))
			for _, s := range stats {
				rows = append(rows, senderRow{
					FromEmail:      s.FromEmail,
					FromName:       s.FromName,
					Count:          s.Count,
					TotalSize:      s.TotalSize,
					UnreadCount:    s.UnreadCount,
					UnreadRate:     s.UnreadRate,
					FirstSeen:      msToRFC3339(s.FirstSeen),
					LastSeen:       msToRFC3339(s.LastSeen),
					HasUnsubscribe: s.HasUnsubscribe,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no local data matched for account %q — populate the store with: gmail-pp-cli sync --account %s\n",
					account, account)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}

	cmd.Flags().IntVar(&top, "top", 25, "Number of senders to return")
	cmd.Flags().StringVar(&category, "category", "", "Filter to one derived category (primary, promotions, social, updates, forums)")
	cmd.Flags().StringVar(&since, "since", "", "Only count messages newer than this (e.g. 90d, 24h, 1w)")
	return cmd
}

// msToRFC3339 renders a ms-since-epoch timestamp as RFC3339 UTC, or "" for 0.
func msToRFC3339(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

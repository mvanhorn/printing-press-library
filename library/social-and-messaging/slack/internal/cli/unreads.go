// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It implements
// `unreads` — a priority-bucketed inventory of unread Slack activity
// computed entirely from the local mirror.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// Unread priority buckets, highest first. A message lands in exactly one
// bucket based on the channel it arrived in.
const (
	bucketDM        = "dm"
	bucketPartner   = "partner_channel"
	bucketInternal  = "internal_channel"
	bucketBroadcast = "broadcast"
)

// unreadBucketOrder is the fixed display/priority order.
var unreadBucketOrder = []string{bucketDM, bucketPartner, bucketInternal, bucketBroadcast}

// classifyChannelBucket assigns a channel to a priority bucket. DMs and
// MPIMs are highest. A channel whose name marks it customer/partner
// facing outranks a plain internal channel; #general / #announcements
// style broadcast channels are lowest.
func classifyChannelBucket(ch store.Channel) string {
	if ch.IsIM || ch.IsMPIM {
		return bucketDM
	}
	name := strings.ToLower(ch.Name)
	switch {
	case strings.Contains(name, "general"),
		strings.Contains(name, "announce"),
		strings.Contains(name, "broadcast"),
		strings.Contains(name, "random"):
		return bucketBroadcast
	case strings.Contains(name, "partner"),
		strings.Contains(name, "client"),
		strings.Contains(name, "customer"),
		strings.HasPrefix(name, "ext-"),
		strings.HasPrefix(name, "shared-"):
		return bucketPartner
	default:
		return bucketInternal
	}
}

// unreadItem is one unread message in the inventory.
type unreadItem struct {
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Bucket      string `json:"bucket"`
	TS          string `json:"ts"`
	UserID      string `json:"user_id"`
	Text        string `json:"text"`
	Permalink   string `json:"permalink"`
	IsThread    bool   `json:"is_thread"`
}

// unreadsReport is the full JSON shape of `unreads`.
type unreadsReport struct {
	TotalUnread  int                     `json:"total_unread"`
	BucketCounts map[string]int          `json:"bucket_counts"`
	Buckets      map[string][]unreadItem `json:"buckets"`
}

// emptyUnreadsReport returns a report with all buckets present but empty,
// so an honest "nothing unread" answer is still well-shaped JSON.
func emptyUnreadsReport() unreadsReport {
	r := unreadsReport{
		BucketCounts: map[string]int{},
		Buckets:      map[string][]unreadItem{},
	}
	for _, b := range unreadBucketOrder {
		r.BucketCounts[b] = 0
		r.Buckets[b] = []unreadItem{}
	}
	return r
}

// collectUnreads walks every channel, treats messages newer than the
// channel's last synced/read cursor as unread, and buckets them. The
// cursor is the mirror's per-channel high-water-mark — the honest
// local-mirror notion of "last seen". A channel never synced has no
// cursor, so all its messages count as unread.
func collectUnreads(ctx context.Context, db *store.Store, channels []store.Channel) (unreadsReport, error) {
	report := emptyUnreadsReport()
	for _, ch := range channels {
		if ch.IsArchived {
			continue
		}
		cursor, err := db.GetChannelCursor(ctx, ch.ID)
		if err != nil {
			return report, err
		}
		// Messages strictly newer than the cursor are unread.
		msgs, err := db.MessagesInWindow(ctx, []string{ch.ID}, advanceTS(cursor), "")
		if err != nil {
			return report, err
		}
		bucket := classifyChannelBucket(ch)
		for _, m := range msgs {
			if cursor != "" && m.TS <= cursor {
				continue
			}
			report.Buckets[bucket] = append(report.Buckets[bucket], unreadItem{
				ChannelID:   ch.ID,
				ChannelName: ch.Name,
				Bucket:      bucket,
				TS:          m.TS,
				UserID:      m.UserID,
				Text:        m.Text,
				Permalink:   m.Permalink,
				IsThread:    m.ThreadTS != "" && m.ThreadTS != m.TS,
			})
		}
	}
	for b, items := range report.Buckets {
		sort.SliceStable(items, func(i, j int) bool { return items[i].TS > items[j].TS })
		report.Buckets[b] = items
		report.BucketCounts[b] = len(items)
		report.TotalUnread += len(items)
	}
	return report, nil
}

// advanceTS returns the smallest ts strictly greater than cursor for use
// as an inclusive lower bound. Slack ts has microsecond precision, so
// appending the smallest increment is awkward; instead the caller also
// re-checks `m.TS <= cursor` after the query, so here we just pass the
// cursor itself through as the inclusive bound and rely on that filter.
func advanceTS(cursor string) string {
	return cursor
}

func newUnreadsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var priority bool
	var limit int

	cmd := &cobra.Command{
		Use:   "unreads",
		Short: "Inventory unread DMs, mentions and threads, bucketed by priority",
		Long: `unreads inventories unread Slack activity from the local mirror,
bucketed by priority: DM > partner channel > internal channel >
broadcast. "Unread" is computed against each channel's per-channel sync
cursor (the mirror's high-water-mark) — messages newer than the cursor.

With --priority, only per-bucket counts are printed (the triage view).
Without it, the full per-bucket message list is returned.

All data is read from the local mirror — run 'slack-pp-cli sync mirror'
first. No live Slack calls are made.`,
		Example: stringTrimNL(`
  # Full unread inventory
  slack-pp-cli unreads --agent

  # Just the per-bucket counts (morning triage)
  slack-pp-cli unreads --priority --agent

  # Preview without touching the database
  slack-pp-cli unreads --dry-run`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'slack-pp-cli sync mirror' first.", err)
			}
			defer db.Close()

			channels, err := db.ListChannels(cmd.Context(), false)
			if err != nil {
				return fmt.Errorf("listing channels: %w", err)
			}
			report, err := collectUnreads(cmd.Context(), db, channels)
			if err != nil {
				return fmt.Errorf("collecting unreads: %w", err)
			}

			// Reading DM/MPIM unread content is an audited event.
			for _, ch := range channels {
				if (ch.IsIM || ch.IsMPIM) && report.BucketCounts[bucketDM] > 0 {
					_ = db.AppendAuditLog(cmd.Context(), auditCaller(), "unreads", ch.ID,
						"unread inventory read of dm/mpim channel")
				}
			}

			if priority {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"total_unread":  report.TotalUnread,
					"bucket_counts": report.BucketCounts,
				}, flags)
			}
			if limit > 0 {
				for b, items := range report.Buckets {
					if len(items) > limit {
						report.Buckets[b] = items[:limit]
					}
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().BoolVar(&priority, "priority", false, "Print only per-bucket counts (triage view)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum items per bucket in the full view")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slack-pp-cli/data.db)")
	return cmd
}

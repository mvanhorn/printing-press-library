// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: what date range the local mirror actually holds per channel.

// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/slackanalytics"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent := findNovelParent(root, []string{"archive"})
		if parent == nil {
			return
		}
		addNovelCommandIfAbsent(parent, newNovelArchiveCoverageCmd(flags))
	})
}

// archiveCoverageRow is one channel's mirrored range and its holes.
type archiveCoverageRow struct {
	Channel                 string  `json:"channel"`
	ChannelName             string  `json:"channel_name"`
	Messages                int     `json:"messages"`
	FirstTS                 string  `json:"first_ts"`
	LastTS                  string  `json:"last_ts"`
	FirstMessage            string  `json:"first_message"`
	LastMessage             string  `json:"last_message"`
	SpanDays                float64 `json:"span_days"`
	LargestGapDays          float64 `json:"largest_gap_days"`
	StaleDays               float64 `json:"stale_days"`
	MessagesBeyondRetention int     `json:"messages_beyond_retention"`
	DaysBeyondRetention     float64 `json:"days_beyond_retention"`
	CoversRetentionWindow   bool    `json:"covers_retention_window"`
}

func newNovelArchiveCoverageCmd(flags *rootFlags) *cobra.Command {
	var flagChannel string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Show what date range your local mirror actually holds per channel, and where the gaps are.",
		Long: `Report the mirrored message range for every channel: oldest and newest
message, total count, the largest hole inside the synced range, how stale the
tail is, and how much of the archive already predates Slack's 90-day retention
wall (data that exists nowhere else).`,
		Example: strings.Trim(`
  # Coverage for every mirrored channel
  slack-pp-cli archive coverage

  # One channel, as JSON
  slack-pp-cli archive coverage --channel C0GENERAL --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would report mirrored date coverage per channel from the local store (no API call, no writes)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: slack-pp-cli sync --resources conversations,users && slack-pp-cli archive sync --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]archiveCoverageRow, 0), flags)
				}
				return nil
			}

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "") {
				hintIfStale(cmd, db, "", flags.maxAge)
			}

			messages, err := loadLocalMessages(ctx, db)
			if err != nil {
				return err
			}
			channels, err := loadLocalChannels(ctx, db)
			if err != nil {
				return err
			}

			warnUnmatchedChannelFilter(cmd.ErrOrStderr(), flagChannel, channels, messages)

			now := time.Now().UTC()
			type bucket struct {
				count           int
				beyond          int
				firstTS, lastTS string
				first, last     time.Time
				times           []time.Time
			}
			buckets := map[string]*bucket{}
			for _, m := range messages {
				if !recallChannelMatches(flagChannel, m.Channel, channels) {
					continue
				}
				b, ok := buckets[m.Channel]
				if !ok {
					b = &bucket{times: make([]time.Time, 0, 16)}
					buckets[m.Channel] = b
				}
				b.count++
				if b.firstTS == "" || m.TS < b.firstTS {
					b.firstTS = m.TS
					b.first = m.At
				}
				if m.TS > b.lastTS {
					b.lastTS = m.TS
					b.last = m.At
				}
				if m.HasTime {
					b.times = append(b.times, m.At)
					if slackanalytics.BeyondRetention(m.At, now, slackanalytics.RetentionWall) {
						b.beyond++
					}
				}
			}

			rows := make([]archiveCoverageRow, 0, len(buckets))
			for channelID, b := range buckets {
				row := archiveCoverageRow{
					Channel:                 channelID,
					ChannelName:             channelLabel(channels, channelID),
					Messages:                b.count,
					FirstTS:                 b.firstTS,
					LastTS:                  b.lastTS,
					FirstMessage:            rfc3339(b.first, !b.first.IsZero()),
					LastMessage:             rfc3339(b.last, !b.last.IsZero()),
					MessagesBeyondRetention: b.beyond,
				}
				if !b.first.IsZero() && !b.last.IsZero() {
					row.SpanDays = slackanalytics.RoundDays(b.last.Sub(b.first))
					row.StaleDays = slackanalytics.RoundDays(now.Sub(b.last))
					row.LargestGapDays = slackanalytics.RoundDays(slackanalytics.LargestGap(b.times))
					// How far past the retention wall the oldest mirrored
					// message reaches — history Slack itself can no longer
					// return.
					wallStart := now.Add(-slackanalytics.RetentionWall)
					if b.first.Before(wallStart) {
						row.DaysBeyondRetention = slackanalytics.RoundDays(wallStart.Sub(b.first))
					}
					// Coverage is "complete" for the retention window when
					// the mirror reaches back at least to the wall and its
					// tail is no more than a day stale.
					row.CoversRetentionWindow = !b.first.After(wallStart) && now.Sub(b.last) <= 24*time.Hour
				}
				rows = append(rows, row)
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Messages != rows[j].Messages {
					return rows[i].Messages > rows[j].Messages
				}
				return rows[i].Channel < rows[j].Channel
			})

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(out, "No mirrored messages yet. Run 'slack-pp-cli sync' to populate the archive.")
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "CHANNEL\tMESSAGES\tOLDEST\tNEWEST\tSPAN (D)\tLARGEST GAP (D)\tSTALE (D)\tPRE-WALL")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%.2f\t%.2f\t%.2f\t%d\n",
					r.ChannelName, r.Messages, r.FirstMessage, r.LastMessage,
					r.SpanDays, r.LargestGapDays, r.StaleDays, r.MessagesBeyondRetention)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagChannel, "channel", "", "Restrict to one channel (ID or #name)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: resolved data directory data.db)")
	return cmd
}

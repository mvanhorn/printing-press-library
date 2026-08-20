// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: score and compare channels from the local mirror.

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
		addNovelCommandIfAbsent(root, newNovelHealthCmd(flags))
	})
}

// channelHealthRow scores one channel. Volume counters are all-time so every
// mirrored channel is always represented; the rate column is scoped to the
// --days window.
type channelHealthRow struct {
	Channel                 string  `json:"channel"`
	ChannelName             string  `json:"channel_name"`
	Messages                int     `json:"messages"`
	MessagesInWindow        int     `json:"messages_in_window"`
	WindowDays              int     `json:"window_days"`
	MessagesPerDay          float64 `json:"messages_per_day"`
	DistinctPosters         int     `json:"distinct_posters"`
	Threads                 int     `json:"threads"`
	MedianFirstReplySeconds float64 `json:"median_first_reply_seconds"`
	MedianFirstReplyHuman   string  `json:"median_first_reply"`
	LastMessage             string  `json:"last_message"`
	DaysIdle                float64 `json:"days_idle"`
	Archived                bool    `json:"archived"`
	Dying                   bool    `json:"dying"`
}

func newNovelHealthCmd(flags *rootFlags) *cobra.Command {
	var flagDying bool
	var flagDays int
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Compare your channels by messages per day, distinct posters, median first-reply latency, and days idle.",
		Long: "Use this command to score and compare channels by volume, distinct posters, median first-reply latency, and idle days; --dying filters to archive candidates. " +
			"Do NOT use this command for raw grouped counts over a time bucket; use 'analytics --type messages --group-by channel' instead.",
		Example: strings.Trim(`
  # Score every mirrored channel over the last 30 days
  slack-pp-cli health --days 30

  # Archive candidates only (idle longer than the window)
  slack-pp-cli health --dying --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would score local-mirror channels by volume, posters, reply latency, and idle days (no API call, no writes)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if flagDays <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be positive, got %d", flagDays))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: slack-pp-cli sync --resources conversations,users && slack-pp-cli archive sync --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]channelHealthRow, 0), flags)
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

			now := time.Now().UTC()
			window := time.Duration(flagDays) * 24 * time.Hour
			cutoff := now.Add(-window)

			type agg struct {
				messages   int
				inWindow   int
				posters    map[string]bool
				lastTS     string
				last       time.Time
				latencies  []time.Duration
				threadKeys map[string]bool
			}
			aggs := map[string]*agg{}
			get := func(channelID string) *agg {
				a, ok := aggs[channelID]
				if !ok {
					a = &agg{
						posters:    map[string]bool{},
						threadKeys: map[string]bool{},
						latencies:  make([]time.Duration, 0, 8),
					}
					aggs[channelID] = a
				}
				return a
			}
			for _, m := range messages {
				a := get(m.Channel)
				a.messages++
				if m.User != "" {
					a.posters[m.User] = true
				}
				if m.TS > a.lastTS {
					a.lastTS = m.TS
					a.last = m.At
				}
				if m.HasTime && !m.At.Before(cutoff) {
					a.inWindow++
				}
			}
			// First-reply latency: parent -> earliest reply, per thread.
			for _, thread := range groupThreads(messages, 2) {
				a := get(thread.Channel)
				a.threadKeys[thread.ThreadTS] = true
				parent, ok := thread.Parent()
				if !ok || !parent.HasTime {
					continue
				}
				for _, reply := range thread.Replies() {
					if !reply.HasTime {
						continue
					}
					if latency := reply.At.Sub(parent.At); latency >= 0 {
						a.latencies = append(a.latencies, latency)
					}
					break // replies are ordered oldest-first
				}
			}

			rows := make([]channelHealthRow, 0, len(aggs))
			for channelID, a := range aggs {
				idle := 0.0
				if !a.last.IsZero() {
					idle = slackanalytics.RoundDays(now.Sub(a.last))
				}
				median := slackanalytics.MedianDuration(a.latencies)
				row := channelHealthRow{
					Channel:                 channelID,
					ChannelName:             channelLabel(channels, channelID),
					Messages:                a.messages,
					MessagesInWindow:        a.inWindow,
					WindowDays:              flagDays,
					MessagesPerDay:          slackanalytics.PerDay(a.inWindow, window),
					DistinctPosters:         len(a.posters),
					Threads:                 len(a.threadKeys),
					MedianFirstReplySeconds: median.Seconds(),
					MedianFirstReplyHuman:   median.String(),
					LastMessage:             rfc3339(a.last, !a.last.IsZero()),
					DaysIdle:                idle,
				}
				if ch, ok := channels[channelID]; ok {
					row.Archived = ch.IsArchived
				}
				// An archive candidate: silent for at least the whole
				// window, or already archived in Slack.
				row.Dying = row.Archived || idle >= float64(flagDays)
				rows = append(rows, row)
			}

			if flagDying {
				filtered := make([]channelHealthRow, 0, len(rows))
				for _, r := range rows {
					if r.Dying {
						filtered = append(filtered, r)
					}
				}
				rows = filtered
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if flagDying {
					if rows[i].DaysIdle != rows[j].DaysIdle {
						return rows[i].DaysIdle > rows[j].DaysIdle
					}
					return rows[i].Channel < rows[j].Channel
				}
				if rows[i].MessagesPerDay != rows[j].MessagesPerDay {
					return rows[i].MessagesPerDay > rows[j].MessagesPerDay
				}
				if rows[i].Messages != rows[j].Messages {
					return rows[i].Messages > rows[j].Messages
				}
				return rows[i].Channel < rows[j].Channel
			})
			limit := flagLimit
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, rows, flags)
			}
			if len(rows) == 0 {
				if flagDying {
					fmt.Fprintln(out, "No archive candidates: every mirrored channel has posted inside the window.")
					return nil
				}
				fmt.Fprintln(out, "No mirrored messages yet. Run 'slack-pp-cli sync' to populate the archive.")
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "CHANNEL\tMSGS\tMSGS/DAY\tPOSTERS\tTHREADS\tMEDIAN 1ST REPLY\tIDLE (D)\tDYING")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%d\t%.2f\t%d\t%d\t%s\t%.2f\t%t\n",
					r.ChannelName, r.Messages, r.MessagesPerDay, r.DistinctPosters,
					r.Threads, r.MedianFirstReplyHuman, r.DaysIdle, r.Dying)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&flagDying, "dying", false, "Only archive candidates: channels idle for at least --days, or already archived")
	cmd.Flags().IntVar(&flagDays, "days", 30, "Window (days) for the messages/day rate and the --dying idle threshold")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum channels to return (0 for all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: resolved data directory data.db)")
	return cmd
}

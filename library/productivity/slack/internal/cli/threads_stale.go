// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: threads across the archive whose last word was not yours.

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
		parent := findNovelParent(root, []string{"threads"})
		if parent == nil {
			return
		}
		addNovelCommandIfAbsent(parent, newNovelThreadsStaleCmd(flags))
	})
}

// staleThreadRow is one unanswered thread, ranked by how long it has sat.
type staleThreadRow struct {
	Channel            string  `json:"channel"`
	ChannelName        string  `json:"channel_name"`
	ThreadTS           string  `json:"thread_ts"`
	Messages           int     `json:"messages"`
	Replies            int     `json:"replies"`
	Participants       int     `json:"participants"`
	ParentUser         string  `json:"parent_user"`
	ParentUserName     string  `json:"parent_user_name"`
	ParentText         string  `json:"parent_text"`
	LastReplyUser      string  `json:"last_reply_user"`
	LastReplyUserName  string  `json:"last_reply_user_name"`
	LastReplyTS        string  `json:"last_reply_ts"`
	LastReply          string  `json:"last_reply"`
	LastReplyText      string  `json:"last_reply_text"`
	AgeDays            float64 `json:"age_days"`
	SelfParticipated   bool    `json:"self_participated"`
	BeyondSlackWall    bool    `json:"beyond_slack_retention"`
	AwaitingFromSelfID string  `json:"awaiting_reply_from"`
}

func newNovelThreadsStaleCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagChannel string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List threads across the archive where the last reply is not yours and nobody has answered since.",
		Long: "Use this command to list unanswered threads across the whole archive ranked by age since last reply. " +
			"Do NOT use this command for a personal since-I-was-away summary; use 'catchup' instead.",
		Example: strings.Trim(`
  # Threads nobody has answered for three days or more
  slack-pp-cli threads stale --days 3

  # Everything currently unanswered in one channel, as JSON
  slack-pp-cli threads stale --days 0 --channel C0GENERAL --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank unanswered local-mirror threads by age since the last reply (no API call, no writes)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if flagDays < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be zero or positive, got %d", flagDays))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: slack-pp-cli sync --resources conversations,users && slack-pp-cli archive sync --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]staleThreadRow, 0), flags)
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

			selfID, _, err := resolveSelfUserID(ctx, db)
			if err != nil {
				return err
			}
			messages, err := loadLocalMessages(ctx, db)
			if err != nil {
				return err
			}
			channels, err := loadLocalChannels(ctx, db)
			if err != nil {
				return err
			}
			users, err := loadLocalUsers(ctx, db)
			if err != nil {
				return err
			}
			if selfID == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "hint: the local mirror carries no authenticated identity, so every unanswered thread is listed. Run: slack-pp-cli sync --resources users-identity --db %s\n", dbPath)
			}

			warnUnmatchedChannelFilter(cmd.ErrOrStderr(), flagChannel, channels, messages)

			// Preview text is de-rendered at the presentation boundary; the
			// mirrored bodies keep Slack's own markup.
			renderer := newTextRenderer(users, channels)

			now := time.Now().UTC()
			minAge := time.Duration(flagDays) * 24 * time.Hour
			limit := flagLimit
			if limit <= 0 {
				limit = 25
			}

			rows := make([]staleThreadRow, 0, 16)
			for _, thread := range groupThreads(messages, 2) {
				if !recallChannelMatches(flagChannel, thread.Channel, channels) {
					continue
				}
				latest := thread.Latest()
				// The defining rule: a thread is stale only when its most
				// recent message came from somebody else. Your own last word
				// means the ball is in their court, not yours.
				if selfID != "" && strings.EqualFold(latest.User, selfID) {
					continue
				}
				age := time.Duration(0)
				if latest.HasTime {
					age = now.Sub(latest.At)
				}
				if minAge > 0 && age < minAge {
					continue
				}
				participants := map[string]bool{}
				selfParticipated := false
				for _, m := range thread.Messages {
					if m.User != "" {
						participants[m.User] = true
					}
					if selfID != "" && strings.EqualFold(m.User, selfID) {
						selfParticipated = true
					}
				}
				row := staleThreadRow{
					Channel:            thread.Channel,
					ChannelName:        channelLabel(channels, thread.Channel),
					ThreadTS:           thread.ThreadTS,
					Messages:           len(thread.Messages),
					Replies:            len(thread.Replies()),
					Participants:       len(participants),
					LastReplyUser:      latest.User,
					LastReplyUserName:  userLabel(users, latest.User),
					LastReplyTS:        latest.TS,
					LastReply:          rfc3339(latest.At, latest.HasTime),
					LastReplyText:      renderer.RenderSnippet(latest.Text, 160),
					AgeDays:            slackanalytics.RoundDays(age),
					SelfParticipated:   selfParticipated,
					BeyondSlackWall:    latest.HasTime && slackanalytics.BeyondRetention(latest.At, now, slackanalytics.RetentionWall),
					AwaitingFromSelfID: selfID,
				}
				if parent, ok := thread.Parent(); ok {
					row.ParentUser = parent.User
					row.ParentUserName = userLabel(users, parent.User)
					row.ParentText = renderer.RenderSnippet(parent.Text, 160)
				}
				rows = append(rows, row)
			}
			// Oldest last-reply first: the thread that has been waiting
			// longest is the one most likely to have been dropped.
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].AgeDays != rows[j].AgeDays {
					return rows[i].AgeDays > rows[j].AgeDays
				}
				return rows[i].LastReplyTS < rows[j].LastReplyTS
			})
			if len(rows) > limit {
				rows = rows[:limit]
			}

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(out, "No unanswered threads in the local mirror.")
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "AGE (D)\tCHANNEL\tLAST REPLY BY\tREPLIES\tTHREAD")
			for _, r := range rows {
				preview := r.ParentText
				if preview == "" {
					preview = r.LastReplyText
				}
				fmt.Fprintf(tw, "%.2f\t%s\t%s\t%d\t%s\n", r.AgeDays, r.ChannelName, r.LastReplyUserName, r.Replies, slackanalytics.Snippet(preview, 70))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 3, "Only threads whose last reply is at least this many days old (0 = all unanswered threads)")
	cmd.Flags().StringVar(&flagChannel, "channel", "", "Restrict to one channel (ID or #name)")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum threads to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: resolved data directory data.db)")
	return cmd
}

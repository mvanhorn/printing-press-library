// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: what happened in the mirrored workspace while you were away.

// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/slackanalytics"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelCatchupCmd(flags))
	})
}

// catchupChannel is per-channel new-message volume inside the window.
type catchupChannel struct {
	Channel      string `json:"channel"`
	ChannelName  string `json:"channel_name"`
	NewMessages  int    `json:"new_messages"`
	LastActivity string `json:"last_activity"`
}

// catchupMention is one message inside the window that addressed you.
type catchupMention struct {
	Channel     string `json:"channel"`
	ChannelName string `json:"channel_name"`
	User        string `json:"user"`
	UserName    string `json:"user_name"`
	Text        string `json:"text"`
	TS          string `json:"ts"`
	Timestamp   string `json:"timestamp"`
	ThreadTS    string `json:"thread_ts"`
	MentionType string `json:"mention_type"`
}

// catchupThread is a thread with fresh activity whose last word was somebody
// else's — i.e. the ball is in your court.
type catchupThread struct {
	Channel           string  `json:"channel"`
	ChannelName       string  `json:"channel_name"`
	ThreadTS          string  `json:"thread_ts"`
	Messages          int     `json:"messages"`
	ParentText        string  `json:"parent_text"`
	LastReplyUser     string  `json:"last_reply_user"`
	LastReplyUserName string  `json:"last_reply_user_name"`
	LastReplyTS       string  `json:"last_reply_ts"`
	LastReply         string  `json:"last_reply"`
	LastReplyText     string  `json:"last_reply_text"`
	LastReplyAgeHours float64 `json:"last_reply_age_hours"`
}

// catchupReport is the whole away-summary. It is emitted as a one-element
// array so every output mode — including the empty-mirror path — renders a
// JSON array rather than switching shapes between runs.
type catchupReport struct {
	Window               string           `json:"window"`
	Since                string           `json:"since"`
	SelfUser             string           `json:"self_user"`
	SelfUserName         string           `json:"self_user_name"`
	SelfUserSource       string           `json:"self_user_source"`
	SelfUsergroups       []string         `json:"self_usergroups"`
	TotalNewMessages     int              `json:"total_new_messages"`
	Channels             []catchupChannel `json:"channels"`
	Mentions             []catchupMention `json:"mentions"`
	ThreadsAwaitingReply []catchupThread  `json:"threads_awaiting_reply"`
	// Note distinguishes "nobody mentioned you" from "we could not work out
	// who you are". Both render as an empty Mentions list, and a caller that
	// cannot tell them apart will read an unresolved identity as a clean
	// inbox. Populated only when the personal half of this report is
	// unavailable.
	Note string `json:"note,omitempty"`
}

func newNovelCatchupCmd(flags *rootFlags) *cobra.Command {
	// --since is a StringVar parsed with cliutil.ParseDurationLoose, not a
	// DurationVar: DurationVar rejects the 7d/4w shorthand the rest of this
	// CLI accepts.
	var flagSince string
	var flagChannel string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "catchup",
		Short: "See what happened while you were away: new volume per channel, messages that mention you, and threads awaiting your reply.",
		Long: "Use this command for \"what happened while I was away\" across channels — new volume, mentions of you, and threads still awaiting your reply — over a time window. " +
			"Do NOT use this command to profile a named third party's posting behavior; use 'users activity' instead.",
		Example: strings.Trim(`
  # Everything since yesterday
  slack-pp-cli catchup --since 24h

  # A long weekend, one channel, as JSON for an agent
  slack-pp-cli catchup --since 3d --channel C0GENERAL --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would summarize new local-mirror activity, mentions, and unanswered threads (no API call, no writes)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if strings.TrimSpace(flagSince) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--since is required (e.g. --since 24h)"))
			}
			window, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}
			if window <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--since must be a positive window, got %q", flagSince))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: slack-pp-cli sync --resources conversations,users && slack-pp-cli archive sync --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]catchupReport, 0), flags)
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

			// One local pass: every section below is computed from these
			// fully-drained result sets, never from a query nested inside an
			// open cursor.
			selfID, selfSource, err := resolveSelfUserID(ctx, db)
			if err != nil {
				return err
			}
			usergroups, err := loadSelfUsergroups(ctx, db, selfID)
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
				fmt.Fprintf(cmd.ErrOrStderr(), "hint: the local mirror carries no authenticated identity, so mentions cannot be attributed. Run: slack-pp-cli sync --resources users-identity --db %s\n", dbPath)
			}

			warnUnmatchedChannelFilter(cmd.ErrOrStderr(), flagChannel, channels, messages)

			// De-markup runs on the way out only: ClassifyMention below still
			// reads the raw body, where Slack's <@Uxxx> encoding lives.
			renderer := newTextRenderer(users, channels)

			now := time.Now().UTC()
			cutoff := now.Add(-window)
			limit := flagLimit
			if limit <= 0 {
				limit = 25
			}

			report := catchupReport{
				Window:               flagSince,
				Since:                cutoff.Format(time.RFC3339),
				SelfUser:             selfID,
				SelfUserName:         userLabel(users, selfID),
				SelfUserSource:       selfSource,
				SelfUsergroups:       usergroups,
				Channels:             make([]catchupChannel, 0, len(channels)),
				Mentions:             make([]catchupMention, 0, 8),
				ThreadsAwaitingReply: make([]catchupThread, 0, 8),
			}
			if selfID == "" {
				// A bot token has no personal identity in the workspace, so
				// "mentions of you" and "threads awaiting your reply" have no
				// subject to resolve against. Say so rather than returning an
				// empty list that reads as "all clear".
				report.Note = "could not resolve your Slack user id from the local mirror, so mentions and awaiting-reply are not personalized; " +
					"sync users and authenticate with a user token (SLACK_USER_TOKEN) for the personal half of this report"
			}

			counts := map[string]int{}
			lastActivity := map[string]time.Time{}
			inWindow := make([]localMessage, 0, len(messages))
			for _, m := range messages {
				if !m.HasTime || m.At.Before(cutoff) {
					continue
				}
				if !recallChannelMatches(flagChannel, m.Channel, channels) {
					continue
				}
				inWindow = append(inWindow, m)
				counts[m.Channel]++
				if prev, ok := lastActivity[m.Channel]; !ok || m.At.After(prev) {
					lastActivity[m.Channel] = m.At
				}
			}

			for channelID, count := range counts {
				report.TotalNewMessages += count
				report.Channels = append(report.Channels, catchupChannel{
					Channel:      channelID,
					ChannelName:  channelLabel(channels, channelID),
					NewMessages:  count,
					LastActivity: rfc3339(lastActivity[channelID], true),
				})
			}
			sort.SliceStable(report.Channels, func(i, j int) bool {
				if report.Channels[i].NewMessages != report.Channels[j].NewMessages {
					return report.Channels[i].NewMessages > report.Channels[j].NewMessages
				}
				return report.Channels[i].Channel < report.Channels[j].Channel
			})
			if len(report.Channels) > limit {
				report.Channels = report.Channels[:limit]
			}

			// Mentions: an explicit <@Uxxx> of the authenticated user, or a
			// <!subteam^Sxxx> of a usergroup they belong to. Your own
			// messages never count as mentions of you.
			for i := len(inWindow) - 1; i >= 0; i-- {
				m := inWindow[i]
				if selfID != "" && strings.EqualFold(m.User, selfID) {
					continue
				}
				kind := slackanalytics.ClassifyMention(m.Text, selfID, usergroups, false)
				if kind == slackanalytics.MentionNone {
					continue
				}
				report.Mentions = append(report.Mentions, catchupMention{
					Channel:     m.Channel,
					ChannelName: channelLabel(channels, m.Channel),
					User:        m.User,
					UserName:    userLabel(users, m.User),
					Text:        renderer.Render(m.Text),
					TS:          m.TS,
					Timestamp:   rfc3339(m.At, m.HasTime),
					ThreadTS:    m.ThreadTS,
					MentionType: string(kind),
				})
				if len(report.Mentions) >= limit {
					break
				}
			}

			// Threads awaiting a reply: fresh activity in the window whose
			// newest message was written by somebody other than you.
			for _, thread := range groupThreads(messages, 2) {
				if !recallChannelMatches(flagChannel, thread.Channel, channels) {
					continue
				}
				latest := thread.Latest()
				if !latest.HasTime || latest.At.Before(cutoff) {
					continue
				}
				if selfID != "" && strings.EqualFold(latest.User, selfID) {
					continue
				}
				parentText := ""
				if parent, ok := thread.Parent(); ok {
					parentText = renderer.RenderSnippet(parent.Text, 160)
				}
				report.ThreadsAwaitingReply = append(report.ThreadsAwaitingReply, catchupThread{
					Channel:           thread.Channel,
					ChannelName:       channelLabel(channels, thread.Channel),
					ThreadTS:          thread.ThreadTS,
					Messages:          len(thread.Messages),
					ParentText:        parentText,
					LastReplyUser:     latest.User,
					LastReplyUserName: userLabel(users, latest.User),
					LastReplyTS:       latest.TS,
					LastReply:         rfc3339(latest.At, latest.HasTime),
					LastReplyText:     renderer.RenderSnippet(latest.Text, 160),
					LastReplyAgeHours: roundHours(now.Sub(latest.At)),
				})
				if len(report.ThreadsAwaitingReply) >= limit {
					break
				}
			}

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, []catchupReport{report}, flags)
			}
			return printCatchupHuman(cmd, report)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Look back this far (e.g. 24h, 3d, 1w)")
	cmd.Flags().StringVar(&flagChannel, "channel", "", "Restrict to one channel (ID or #name)")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum rows per section")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: resolved data directory data.db)")
	return cmd
}

// roundHours renders a duration as one-decimal hours for human scanning.
func roundHours(d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(int(d.Hours()*10+0.5)) / 10
}

func printCatchupHuman(cmd *cobra.Command, report catchupReport) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Since %s (%s): %d new messages\n\n", report.Window, report.Since, report.TotalNewMessages)

	if len(report.Channels) == 0 {
		fmt.Fprintln(out, "No new messages in the window.")
	} else {
		tw := newTabWriter(out)
		fmt.Fprintln(tw, "CHANNEL\tNEW\tLAST ACTIVITY")
		for _, c := range report.Channels {
			fmt.Fprintf(tw, "%s\t%d\t%s\n", c.ChannelName, c.NewMessages, c.LastActivity)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	fmt.Fprintln(out, "\nMentions of you:")
	if len(report.Mentions) == 0 {
		fmt.Fprintln(out, "  none")
	} else {
		tw := newTabWriter(out)
		fmt.Fprintln(tw, "WHEN\tCHANNEL\tFROM\tKIND\tTEXT")
		for _, m := range report.Mentions {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", m.Timestamp, m.ChannelName, m.UserName, m.MentionType, slackanalytics.Snippet(m.Text, 70))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	fmt.Fprintln(out, "\nThreads awaiting your reply:")
	if len(report.ThreadsAwaitingReply) == 0 {
		fmt.Fprintln(out, "  none")
		return nil
	}
	tw := newTabWriter(out)
	fmt.Fprintln(tw, "CHANNEL\tLAST REPLY\tBY\tAGE (H)\tTHREAD")
	for _, t := range report.ThreadsAwaitingReply {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%.1f\t%s\n", t.ChannelName, t.LastReply, t.LastReplyUserName, t.LastReplyAgeHours, slackanalytics.Snippet(t.ParentText, 60))
	}
	return tw.Flush()
}

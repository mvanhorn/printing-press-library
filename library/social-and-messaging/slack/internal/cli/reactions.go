// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It implements the
// `reactions` parent command and its `summarize` subcommand — a
// local-mirror GROUP BY over m_reactions that surfaces the messages a
// channel reacted to most over a window.

package cli

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// emojiClass buckets a reaction emoji name into one of four fixed
// sentiment classes. The mapping is intentionally tiny and stable: it is
// a coarse pulse signal, not a sentiment model.
func emojiClass(emoji string) string {
	switch emoji {
	case "tada", "party", "confetti_ball":
		return "celebrate"
	case "+1", "thumbsup", "white_check_mark", "heavy_check_mark", "ok_hand":
		return "approve"
	case "cry", "sob", "disappointed", "white_frowning_face", "slightly_frowning_face":
		return "sad"
	case "fire", "rocket", "100":
		return "hot"
	default:
		return "other"
	}
}

// emojiClasses is the fixed bucket set, in display order.
var emojiClasses = []string{"celebrate", "approve", "sad", "hot", "other"}

// reactionTopMessage is one ranked message in the summary.
type reactionTopMessage struct {
	TS            string `json:"ts"`
	UserID        string `json:"user_id"`
	Text          string `json:"text"`
	Permalink     string `json:"permalink"`
	ReactionCount int    `json:"reaction_count"`
}

// reactionsSummary is the full JSON shape of `reactions summarize`.
type reactionsSummary struct {
	Channel        string             `json:"channel"`
	ChannelID      string             `json:"channel_id"`
	Window         string             `json:"window"`
	TotalReactions int                `json:"total_reactions"`
	TopMessages    []reactionTopMessage `json:"top_messages"`
	EmojiDistribution map[string]int  `json:"emoji_distribution"`
	ClassCounts    map[string]int     `json:"class_counts"`
}

// summarizeReactions aggregates reactions for the messages of one
// channel that fall inside [since, until]. Pure aggregation logic kept
// separate from the cobra plumbing so it is unit-testable.
func summarizeReactions(msgs []store.Message, reactions []store.Reaction, since, until string) reactionsSummary {
	// Window-filter messages, keep the keep-set of ts values.
	inWindow := map[string]store.Message{}
	for _, m := range msgs {
		if since != "" && m.TS < since {
			continue
		}
		if until != "" && m.TS > until {
			continue
		}
		inWindow[m.TS] = m
	}

	perMsg := map[string]int{}
	emojiDist := map[string]int{}
	classCounts := map[string]int{}
	for _, c := range emojiClasses {
		classCounts[c] = 0
	}
	total := 0
	for _, r := range reactions {
		if _, ok := inWindow[r.MessageTS]; !ok {
			continue
		}
		perMsg[r.MessageTS] += r.Count
		emojiDist[r.EmojiName] += r.Count
		classCounts[emojiClass(r.EmojiName)] += r.Count
		total += r.Count
	}

	top := make([]reactionTopMessage, 0, len(perMsg))
	for ts, count := range perMsg {
		m := inWindow[ts]
		top = append(top, reactionTopMessage{
			TS:            ts,
			UserID:        m.UserID,
			Text:          m.Text,
			Permalink:     m.Permalink,
			ReactionCount: count,
		})
	}
	// Rank by reaction count desc, ties broken newest-first for stability.
	sort.SliceStable(top, func(i, j int) bool {
		if top[i].ReactionCount != top[j].ReactionCount {
			return top[i].ReactionCount > top[j].ReactionCount
		}
		return top[i].TS > top[j].TS
	})

	return reactionsSummary{
		TotalReactions:    total,
		TopMessages:       top,
		EmojiDistribution: emojiDist,
		ClassCounts:       classCounts,
	}
}

func newReactionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reactions",
		Short: "Inspect emoji reactions in the local mirror",
		Long: `reactions groups emoji-reaction data from the local Slack mirror.

Subcommands:
  summarize   Aggregate reactions across a channel over a time window.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		// Parent shows help; it is not an alias for any subcommand.
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newReactionsSummarizeCmd(flags))
	return cmd
}

func newReactionsSummarizeCmd(flags *rootFlags) *cobra.Command {
	var channel, window, dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "summarize",
		Short: "Aggregate emoji reactions across a channel over a window",
		Long: `summarize aggregates every emoji reaction on messages in a channel
over a time window: the top messages by reaction count, the emoji
distribution, and a fixed celebrate/approve/sad/hot/other class count.

All data is read from the local mirror — run 'slack-pp-cli sync mirror'
first. No live Slack calls are made.`,
		Example: stringTrimNL(`
  # Last 7 days of reactions in #the-wolf-of-atom
  slack-pp-cli reactions summarize --channel "#the-wolf-of-atom" --window 7d --agent

  # Top 5 most-reacted messages
  slack-pp-cli reactions summarize --channel C0123 --window 14d --limit 5

  # Preview without touching the database
  slack-pp-cli reactions summarize --channel "#general" --dry-run`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if channel == "" {
				return usageErr(fmt.Errorf("--channel is required (a channel name or ID)"))
			}
			if dryRunOK(flags) {
				return nil
			}
			since, until, err := windowBounds(window)
			if err != nil {
				return usageErr(err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'slack-pp-cli sync mirror' first.", err)
			}
			defer db.Close()

			ch, err := db.ResolveChannel(cmd.Context(), channel)
			if err != nil {
				if err == sql.ErrNoRows {
					return notFoundErr(fmt.Errorf("channel %q not found in the mirror; run 'slack-pp-cli sync mirror' or check the name", channel))
				}
				return usageErr(err)
			}

			msgs, err := db.MessagesInWindow(cmd.Context(), []string{ch.ID}, since, until)
			if err != nil {
				return fmt.Errorf("reading messages: %w", err)
			}
			reactions, err := db.ReactionsForChannel(cmd.Context(), ch.ID)
			if err != nil {
				return fmt.Errorf("reading reactions: %w", err)
			}

			summary := summarizeReactions(msgs, reactions, since, until)
			summary.Channel = ch.Name
			summary.ChannelID = ch.ID
			summary.Window = window
			if limit > 0 && len(summary.TopMessages) > limit {
				summary.TopMessages = summary.TopMessages[:limit]
			}

			if ch.IsIM || ch.IsMPIM {
				_ = db.AppendAuditLog(cmd.Context(), auditCaller(), "reactions summarize", ch.ID,
					"reaction summary read of dm/mpim channel")
			}

			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or ID to summarize (required)")
	cmd.Flags().StringVar(&window, "window", "7d", "Time window (e.g. 7d, 24h, 1w)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum top messages to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slack-pp-cli/data.db)")
	return cmd
}

// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built v1.1 novel verb: thread-summary. Pulls a thread from the
// mirror, lists messages with resolved author names, and surfaces a
// bullet / decisions / action-items shape.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// threadMessage is one rendered message in a thread-summary.
type threadMessage struct {
	TS     string `json:"ts"`
	Time   string `json:"time"`
	Author string `json:"author"`
	Text   string `json:"text"`
}

// threadSummary is the full thread-summary result shape.
type threadSummary struct {
	Channel     string          `json:"channel"`
	ChannelID   string          `json:"channel_id"`
	ParentTS    string          `json:"parent_ts"`
	MessageCount int            `json:"message_count"`
	Participants []string       `json:"participants"`
	Decisions    []string        `json:"decisions"`
	ActionItems  []string        `json:"action_items"`
	Messages     []threadMessage `json:"messages"`
}

func newThreadSummaryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "thread-summary [channel] [ts]",
		Short: "Summarize a thread from the mirror: messages, decisions, action items",
		Long: `Pull a thread from the local mirror by channel and parent ts, list
every message with the author resolved to a real name, and surface a
decisions / action-items breakdown derived from message keywords.

Run 'slack-pp-cli sync mirror' first to populate the mirror.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Summarize a thread in #churnsales
  slack-pp-cli thread-summary "#churnsales" 1747000000.001200

  # By channel id, JSON output
  slack-pp-cli thread-summary C0123ABCD 1747000000.001200 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			channelArg, parentTS := strings.TrimSpace(args[0]), strings.TrimSpace(args[1])
			if channelArg == "" || parentTS == "" {
				return usageErr(fmt.Errorf("both [channel] and [ts] arguments are required"))
			}

			ctx := cmd.Context()
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			ch, err := resolveChannelArg(ctx, db, channelArg)
			if err != nil {
				return err
			}
			// Reading an IM/MPIM thread is an audited event.
			if ch.IsIM || ch.IsMPIM {
				_ = db.AppendAuditLog(ctx, "thread-summary", "thread-summary", ch.ID,
					fmt.Sprintf("thread read of %s in DM channel", parentTS))
			}

			summary, err := buildThreadSummary(ctx, db, ch, parentTS)
			if err != nil {
				return err
			}
			if summary.MessageCount == 0 {
				return notFoundErr(fmt.Errorf("no thread at %s in %s in the local mirror", parentTS, channelLabel(ch)))
			}
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Mirror database path (default: ~/.local/share/slack-pp-cli/data.db)")
	return cmd
}

// decisionMarkers / actionMarkers are substring cues used to bucket a
// message into the decisions / action-items lists. Kept small and
// keyword-based — no NLP — so the verb is deterministic and offline.
var decisionMarkers = []string{"decided", "decision", "agreed", "we'll go with", "approved", "let's do", "decidimos", "acordamos"}
var actionMarkers = []string{"todo", "to-do", "action item", "i'll ", "will follow up", "next step", "assign", "pendiente", "voy a ", "hare ", "haré "}

// buildThreadSummary loads the thread, renders each message, collects
// participants, and buckets decision / action-item lines.
func buildThreadSummary(ctx context.Context, db *store.Store, ch store.Channel, parentTS string) (threadSummary, error) {
	replies, err := db.ThreadReplies(ctx, ch.ID, parentTS)
	if err != nil {
		return threadSummary{}, fmt.Errorf("reading thread %s/%s: %w", ch.ID, parentTS, err)
	}
	resolver := newUserNameResolver(ctx, db)
	sum := threadSummary{
		Channel:      channelLabel(ch),
		ChannelID:    ch.ID,
		ParentTS:     parentTS,
		MessageCount: len(replies),
		Participants: []string{},
		Decisions:    []string{},
		ActionItems:  []string{},
		Messages:     []threadMessage{},
	}
	seen := map[string]bool{}
	for _, m := range replies {
		author := resolver.name(m.UserID)
		sum.Messages = append(sum.Messages, threadMessage{
			TS:     m.TS,
			Time:   slackTSToTime(m.TS).UTC().Format("2006-01-02 15:04"),
			Author: author,
			Text:   m.Text,
		})
		if author != "" && !seen[author] {
			seen[author] = true
			sum.Participants = append(sum.Participants, author)
		}
		line := strings.TrimSpace(m.Text)
		if line == "" {
			continue
		}
		if matchesAny(line, decisionMarkers) {
			sum.Decisions = append(sum.Decisions, fmt.Sprintf("%s: %s", author, line))
		}
		if matchesAny(line, actionMarkers) {
			sum.ActionItems = append(sum.ActionItems, fmt.Sprintf("%s: %s", author, line))
		}
	}
	return sum, nil
}

// matchesAny reports whether text contains any of the markers,
// case-insensitively.
func matchesAny(text string, markers []string) bool {
	lower := strings.ToLower(text)
	for _, mk := range markers {
		if strings.Contains(lower, mk) {
			return true
		}
	}
	return false
}

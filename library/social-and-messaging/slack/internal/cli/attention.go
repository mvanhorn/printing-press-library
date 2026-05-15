// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built v1.1 novel verb: attention. A composed inbox-zero triage —
// stale threads, quiet DMs, and attention-flagged reactions, bucketed by
// priority.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// attentionItem is one triage row.
type attentionItem struct {
	Bucket    string `json:"bucket"`
	Kind      string `json:"kind"`
	Channel   string `json:"channel"`
	ChannelID string `json:"channel_id"`
	TS        string `json:"ts"`
	Time      string `json:"time"`
	Detail    string `json:"detail"`
	Permalink string `json:"permalink,omitempty"`
}

// attentionResult is the bucketed triage output.
type attentionResult struct {
	Window  string          `json:"window"`
	Total   int             `json:"total"`
	Buckets map[string]int  `json:"buckets"`
	Items   []attentionItem `json:"items"`
}

// attentionEmojis are reaction names that flag a message as needing
// attention. Bucket priority: DM > drift > flagged-reaction.
var attentionEmojis = map[string]bool{
	"eyes": true, "raised_hand": true, "warning": true, "rotating_light": true,
	"sos": true, "exclamation": true, "bangbang": true, "red_circle": true,
}

func newAttentionCmd(flags *rootFlags) *cobra.Command {
	var window string
	var dbPath string
	var redact bool
	var limit int

	cmd := &cobra.Command{
		Use:   "attention",
		Short: "Composed inbox-zero triage: stale threads, quiet DMs, flagged reactions",
		Long: `Compose an inbox-zero triage list from three signals over a window:

  • dm        — DM channels with messages you may not have answered
  • drift     — threads that have gone stale (no reply past the cutoff)
  • flagged   — messages carrying an attention reaction (:eyes:, :warning:)

Items are bucketed by priority — DM over drift over flagged — so the top
of the list is what to handle first.

Run 'slack-pp-cli sync mirror' first to populate the mirror.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Triage the last 7 days
  slack-pp-cli attention --window 7d

  # JSON for an agent inbox-zero loop
  slack-pp-cli attention --window 3d --json

  # Team-shareable triage with comp/HR keywords stripped
  slack-pp-cli attention --window 7d --redact-sensitivity
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			since, err := resolveWindowTS(window)
			if err != nil {
				return usageErr(err)
			}

			ctx := cmd.Context()
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			result, err := buildAttention(ctx, db, window, since, redact, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&window, "window", p1WindowDefault, "Triage window (e.g. 3d, 7d, 24h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Mirror database path (default: ~/.local/share/slack-pp-cli/data.db)")
	cmd.Flags().BoolVar(&redact, "redact-sensitivity", false, "Strip comp/HR keywords from item detail before output")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum triage items to return")
	return cmd
}

// buildAttention composes the three triage signals into one bucketed
// list. Bucket order is fixed: dm, drift, flagged.
func buildAttention(ctx context.Context, db *store.Store, window, since string, redact bool, limit int) (attentionResult, error) {
	channels, err := db.ListChannels(ctx, false)
	if err != nil {
		return attentionResult{}, err
	}
	labels := channelIDLabels(channels)
	resolver := newUserNameResolver(ctx, db)
	until := nowSlackTS()

	result := attentionResult{
		Window:  strings.TrimSpace(window),
		Buckets: map[string]int{"dm": 0, "drift": 0, "flagged": 0},
		Items:   []attentionItem{},
	}
	if result.Window == "" {
		result.Window = p1WindowDefault
	}

	// Bucket 1 — DMs with in-window activity. An audited read.
	for _, ch := range channels {
		if !ch.IsIM && !ch.IsMPIM {
			continue
		}
		_ = db.AppendAuditLog(ctx, "attention", "attention", ch.ID, "DM scan for triage")
		msgs, merr := db.MessagesInWindow(ctx, []string{ch.ID}, since, until)
		if merr != nil {
			return attentionResult{}, fmt.Errorf("attention DM scan %s: %w", ch.ID, merr)
		}
		if len(msgs) == 0 {
			continue
		}
		last := msgs[len(msgs)-1]
		result.Items = append(result.Items, attentionItem{
			Bucket:    "dm",
			Kind:      "dm",
			Channel:   labelOr(labels, ch.ID),
			ChannelID: ch.ID,
			TS:        last.TS,
			Time:      slackTSToTime(last.TS).UTC().Format("2006-01-02 15:04"),
			Detail:    fmt.Sprintf("%d message(s) — last from %s: %s", len(msgs), resolver.name(last.UserID), maybeRedact(last.Text, redact)),
			Permalink: last.Permalink,
		})
	}

	// Bucket 2 — stale threads (drift).
	stale, serr := db.StaleThreads(ctx, nil, since)
	if serr != nil {
		return attentionResult{}, fmt.Errorf("attention drift scan: %w", serr)
	}
	for _, th := range stale {
		item := attentionItem{
			Bucket:    "drift",
			Kind:      "stale_thread",
			Channel:   labelOr(labels, th.ChannelID),
			ChannelID: th.ChannelID,
			TS:        th.LastReplyTS,
			Time:      slackTSToTime(th.LastReplyTS).UTC().Format("2006-01-02 15:04"),
			Detail:    fmt.Sprintf("thread %s stale since last reply (%d replies)", th.ParentTS, th.ReplyCount),
		}
		if replies, rerr := db.ThreadReplies(ctx, th.ChannelID, th.ParentTS); rerr == nil {
			for _, m := range replies {
				if m.TS == th.ParentTS {
					item.Detail = fmt.Sprintf("stale thread by %s: %s", resolver.name(m.UserID), maybeRedact(m.Text, redact))
					item.Permalink = m.Permalink
					break
				}
			}
		}
		result.Items = append(result.Items, item)
	}

	// Bucket 3 — messages carrying an attention reaction.
	for _, ch := range channels {
		reactions, rerr := db.ReactionsForChannel(ctx, ch.ID)
		if rerr != nil {
			return attentionResult{}, fmt.Errorf("attention reaction scan %s: %w", ch.ID, rerr)
		}
		for _, rx := range reactions {
			if !attentionEmojis[rx.EmojiName] {
				continue
			}
			if since != "" && rx.MessageTS < since {
				continue
			}
			result.Items = append(result.Items, attentionItem{
				Bucket:    "flagged",
				Kind:      "flagged_reaction",
				Channel:   labelOr(labels, ch.ID),
				ChannelID: ch.ID,
				TS:        rx.MessageTS,
				Time:      slackTSToTime(rx.MessageTS).UTC().Format("2006-01-02 15:04"),
				Detail:    fmt.Sprintf(":%s: x%d on message %s", rx.EmojiName, rx.Count, rx.MessageTS),
			})
		}
	}

	// Bucket-priority order: dm, drift, flagged; newest first within.
	bucketRank := map[string]int{"dm": 0, "drift": 1, "flagged": 2}
	sortAttention(result.Items, bucketRank)
	for _, it := range result.Items {
		result.Buckets[it.Bucket]++
	}
	if limit > 0 && len(result.Items) > limit {
		result.Items = result.Items[:limit]
	}
	result.Total = len(result.Items)
	return result, nil
}

// sortAttention orders items by bucket priority, then newest ts first.
func sortAttention(items []attentionItem, rank map[string]int) {
	// Simple insertion sort keeps the dependency surface zero and the
	// item count here is small (triage lists, not full histories).
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && attentionLess(items[j], items[j-1], rank) {
			items[j], items[j-1] = items[j-1], items[j]
			j--
		}
	}
}

func attentionLess(a, b attentionItem, rank map[string]int) bool {
	if rank[a.Bucket] != rank[b.Bucket] {
		return rank[a.Bucket] < rank[b.Bucket]
	}
	return a.TS > b.TS
}

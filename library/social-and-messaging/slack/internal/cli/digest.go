// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built v1.1 novel verb: digest. A multi-channel weekly digest —
// message volume and notable threads per channel over a window.

package cli

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// digestThread is a notable thread inside a channel digest.
type digestThread struct {
	ParentTS   string `json:"parent_ts"`
	ReplyCount int    `json:"reply_count"`
	StartedBy  string `json:"started_by"`
	Text       string `json:"text"`
	Permalink  string `json:"permalink,omitempty"`
}

// digestChannel is one channel's slice of the digest.
type digestChannel struct {
	Channel        string         `json:"channel"`
	ChannelID      string         `json:"channel_id"`
	MessageCount   int            `json:"message_count"`
	Participants   int            `json:"participants"`
	NotableThreads []digestThread `json:"notable_threads"`
}

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var window string
	var dbPath string
	var channelsCSV string
	var redact bool
	var markRead bool
	var topThreads int

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Multi-channel weekly digest: message volume and notable threads",
		Long: `Build a per-channel digest over a time window: message count, distinct
participants, and the most-replied threads. With no --channels, every
channel you are a member of is included.

--mark-read calls conversations.mark on each digested channel so a cron
digest can also clear your unread badges. --mark-read is a write — it is
skipped under --dry-run and the printing-press verifier.

Run 'slack-pp-cli sync mirror' first to populate the mirror.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Weekly digest of all member channels
  slack-pp-cli digest --window 7d

  # Digest of three named channels, JSON
  slack-pp-cli digest --channels churnsales,csm,data --json

  # Digest and mark each channel read (cron inbox-zero)
  slack-pp-cli digest --window 7d --mark-read

  # Team-shareable digest with comp/HR keywords stripped
  slack-pp-cli digest --window 7d --redact-sensitivity
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

			channels, err := digestChannels(ctx, db, channelsCSV)
			if err != nil {
				return err
			}

			digest, err := buildDigest(ctx, db, channels, since, topThreads, redact)
			if err != nil {
				return err
			}

			// --mark-read is a write; honor verify/dry-run short-circuits.
			if markRead && !cliutil.IsVerifyEnv() {
				if err := markChannelsRead(cmd, flags, channels); err != nil {
					return err
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), digest, flags)
		},
	}

	cmd.Flags().StringVar(&window, "window", p1WindowDefault, "Digest window (e.g. 7d, 14d, 24h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Mirror database path (default: ~/.local/share/slack-pp-cli/data.db)")
	cmd.Flags().StringVar(&channelsCSV, "channels", "", "Comma-separated channel ids/names (default: all member channels)")
	cmd.Flags().BoolVar(&redact, "redact-sensitivity", false, "Strip comp/HR keywords from thread text before output")
	cmd.Flags().BoolVar(&markRead, "mark-read", false, "Call conversations.mark on each digested channel (a write)")
	cmd.Flags().IntVar(&topThreads, "top-threads", 3, "Notable threads to surface per channel")
	return cmd
}

// digestChannels resolves the channel set for a digest: the explicit
// --channels list, or every member channel when the flag is empty.
func digestChannels(ctx context.Context, db *store.Store, csv string) ([]store.Channel, error) {
	if strings.TrimSpace(csv) == "" {
		return db.ListChannels(ctx, true)
	}
	var out []store.Channel
	for _, tok := range strings.Split(csv, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		ch, err := resolveChannelArg(ctx, db, tok)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, nil
}

// buildDigest computes per-channel volume + notable threads over the
// window.
func buildDigest(ctx context.Context, db *store.Store, channels []store.Channel, since string, topThreads int, redact bool) ([]digestChannel, error) {
	resolver := newUserNameResolver(ctx, db)
	until := nowSlackTS()
	out := make([]digestChannel, 0, len(channels))
	for _, ch := range channels {
		msgs, err := db.MessagesInWindow(ctx, []string{ch.ID}, since, until)
		if err != nil {
			return nil, fmt.Errorf("digest for %s: %w", ch.ID, err)
		}
		dc := digestChannel{
			Channel:        channelLabel(ch),
			ChannelID:      ch.ID,
			MessageCount:   len(msgs),
			NotableThreads: []digestThread{},
		}
		participants := map[string]bool{}
		var threads []store.Message
		for _, m := range msgs {
			if m.UserID != "" {
				participants[m.UserID] = true
			}
			// A thread root in-window is a notable-thread candidate.
			isRoot := m.ThreadTS == "" || m.ThreadTS == m.TS
			if isRoot && m.ReplyCount > 0 {
				threads = append(threads, m)
			}
		}
		dc.Participants = len(participants)
		sort.Slice(threads, func(i, j int) bool { return threads[i].ReplyCount > threads[j].ReplyCount })
		for i, m := range threads {
			if topThreads > 0 && i >= topThreads {
				break
			}
			dc.NotableThreads = append(dc.NotableThreads, digestThread{
				ParentTS:   m.TS,
				ReplyCount: m.ReplyCount,
				StartedBy:  resolver.name(m.UserID),
				Text:       maybeRedact(m.Text, redact),
				Permalink:  m.Permalink,
			})
		}
		out = append(out, dc)
	}
	return out, nil
}

// markChannelsRead calls conversations.mark on each channel at its newest
// message ts so unread badges clear. Failures are non-fatal — a digest
// that produced output should not fail because one mark call 4xx'd.
func markChannelsRead(cmd *cobra.Command, flags *rootFlags, channels []store.Channel) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	for _, ch := range channels {
		fields := url.Values{}
		fields.Set("channel", ch.ID)
		fields.Set("ts", nowSlackTS())
		if _, _, perr := c.PostForm("/conversations.mark", fields); perr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: conversations.mark for %s failed: %v\n", channelLabel(ch), perr)
		}
	}
	return nil
}

// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built v1.1 novel verb: drift. Threads with no reply in N days —
// the stale-thread / dropped-conversation detector.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// driftThread is one stale-thread row in a drift result.
type driftThread struct {
	Channel     string `json:"channel"`
	ChannelID   string `json:"channel_id"`
	ParentTS    string `json:"parent_ts"`
	LastReply   string `json:"last_reply"`
	StaleDays   int    `json:"stale_days"`
	ReplyCount  int    `json:"reply_count"`
	ParentText  string `json:"parent_text"`
	ParentBy    string `json:"parent_by"`
	Permalink   string `json:"permalink,omitempty"`
}

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var window string
	var dbPath string
	var channelsCSV string
	var limit int

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Threads with no reply in N days — the dropped-conversation detector",
		Long: `Surface threads that have gone stale: threads whose last reply is older
than the --window cutoff. Use it as a drift detector before an L10 or a
weekly review to catch conversations that were dropped mid-flight.

Run 'slack-pp-cli sync mirror' first to populate the mirror.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Threads with no reply in the last 7 days
  slack-pp-cli drift --window 7d

  # Drift in #churnsales and #csm only, JSON
  slack-pp-cli drift --channels churnsales,csm --json

  # Threads stale for over 30 days
  slack-pp-cli drift --window 30d --limit 50
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cutoff, err := resolveWindowTS(window)
			if err != nil {
				return usageErr(err)
			}

			ctx := cmd.Context()
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			var chanIDs []string
			if strings.TrimSpace(channelsCSV) != "" {
				chanIDs, err = resolveChannelList(ctx, db, channelsCSV)
				if err != nil {
					return err
				}
			}

			threads, err := gatherDrift(ctx, db, chanIDs, cutoff, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), threads, flags)
		},
	}

	cmd.Flags().StringVar(&window, "window", p1WindowDefault, "Staleness cutoff (e.g. 7d, 14d, 30d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Mirror database path (default: ~/.local/share/slack-pp-cli/data.db)")
	cmd.Flags().StringVar(&channelsCSV, "channels", "", "Comma-separated channel ids/names to scan (default: all)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum stale threads to return")
	return cmd
}

// gatherDrift collects stale threads, resolves their parent message
// context, and returns them oldest-first (most-drifted at the top).
func gatherDrift(ctx context.Context, db *store.Store, chanIDs []string, cutoff string, limit int) ([]driftThread, error) {
	stale, err := db.StaleThreads(ctx, chanIDs, cutoff)
	if err != nil {
		return nil, fmt.Errorf("scanning stale threads: %w", err)
	}
	channels, err := db.ListChannels(ctx, false)
	if err != nil {
		return nil, err
	}
	labels := channelIDLabels(channels)
	resolver := newUserNameResolver(ctx, db)
	cutoffTime := slackTSToTime(cutoff)

	out := make([]driftThread, 0, len(stale))
	for _, th := range stale {
		dt := driftThread{
			Channel:    labelOr(labels, th.ChannelID),
			ChannelID:  th.ChannelID,
			ParentTS:   th.ParentTS,
			LastReply:  slackTSToTime(th.LastReplyTS).UTC().Format("2006-01-02 15:04"),
			ReplyCount: th.ReplyCount,
		}
		if lr := slackTSToTime(th.LastReplyTS); !lr.IsZero() && !cutoffTime.IsZero() {
			dt.StaleDays = int(cutoffTime.Sub(lr).Hours() / 24)
			if dt.StaleDays < 0 {
				dt.StaleDays = 0
			}
		}
		// Resolve parent message text/author for context.
		replies, rerr := db.ThreadReplies(ctx, th.ChannelID, th.ParentTS)
		if rerr == nil {
			for _, m := range replies {
				if m.TS == th.ParentTS {
					dt.ParentText = m.Text
					dt.ParentBy = resolver.name(m.UserID)
					dt.Permalink = m.Permalink
					break
				}
			}
		}
		out = append(out, dt)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// labelOr returns the label for id, or the raw id when unmapped.
func labelOr(labels map[string]string, id string) string {
	if l := labels[id]; l != "" {
		return l
	}
	return id
}

// resolveChannelList resolves a comma-separated channel list to ids,
// returning a typed error on the first unresolvable token.
func resolveChannelList(ctx context.Context, db *store.Store, csv string) ([]string, error) {
	var ids []string
	for _, tok := range strings.Split(csv, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		ch, err := resolveChannelArg(ctx, db, tok)
		if err != nil {
			return nil, err
		}
		ids = append(ids, ch.ID)
	}
	return ids, nil
}

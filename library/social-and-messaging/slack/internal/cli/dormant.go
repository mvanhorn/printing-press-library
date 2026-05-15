// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built v1.1 novel verb: dormant. Channels with no messages in N
// days — a workspace-cleanup signal.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// dormantChannel is one row of a dormant result.
type dormantChannel struct {
	Channel     string `json:"channel"`
	ChannelID   string `json:"channel_id"`
	LastMessage string `json:"last_message"`
	DormantDays int    `json:"dormant_days"`
	NumMembers  int    `json:"num_members"`
	IsArchived  bool   `json:"is_archived"`
}

func newDormantCmd(flags *rootFlags) *cobra.Command {
	var window string
	var dbPath string
	var includeArchived bool
	var limit int

	cmd := &cobra.Command{
		Use:   "dormant",
		Short: "Channels with no messages in N days — a workspace-cleanup signal",
		Long: `List channels that have had no message in the last --window days. Use
it to find channels worth archiving. Archived channels are excluded by
default; pass --include-archived to keep them.

Run 'slack-pp-cli sync mirror' first to populate the mirror.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Channels silent for 30+ days
  slack-pp-cli dormant --window 30d

  # Include already-archived channels, JSON
  slack-pp-cli dormant --window 90d --include-archived --json

  # Top 20 most-dormant channels
  slack-pp-cli dormant --window 14d --limit 20
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

			rows, err := gatherDormant(ctx, db, cutoff, includeArchived, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}

	cmd.Flags().StringVar(&window, "window", "30d", "Dormancy cutoff (e.g. 14d, 30d, 90d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Mirror database path (default: ~/.local/share/slack-pp-cli/data.db)")
	cmd.Flags().BoolVar(&includeArchived, "include-archived", false, "Include already-archived channels")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum dormant channels to return")
	return cmd
}

// gatherDormant finds channels whose newest mirrored message predates the
// cutoff (or that have no messages at all), most-dormant first.
func gatherDormant(ctx context.Context, db *store.Store, cutoff string, includeArchived bool, limit int) ([]dormantChannel, error) {
	channels, err := db.ListChannels(ctx, false)
	if err != nil {
		return nil, err
	}
	cutoffTime := slackTSToTime(cutoff)

	out := make([]dormantChannel, 0)
	for _, ch := range channels {
		if ch.IsArchived && !includeArchived {
			continue
		}
		// All messages up to the cutoff, ascending — the last one is the
		// channel's newest known message.
		msgs, merr := db.MessagesInWindow(ctx, []string{ch.ID}, "", "")
		if merr != nil {
			return nil, fmt.Errorf("reading messages for %s: %w", ch.ID, merr)
		}
		var lastTS string
		if len(msgs) > 0 {
			lastTS = msgs[len(msgs)-1].TS
		}
		// Dormant when the newest message is older than the cutoff, or
		// when there are no messages at all.
		if lastTS != "" && lastTS >= cutoff {
			continue
		}
		dc := dormantChannel{
			Channel:    channelLabel(ch),
			ChannelID:  ch.ID,
			NumMembers: ch.NumMembers,
			IsArchived: ch.IsArchived,
		}
		if lastTS != "" {
			lt := slackTSToTime(lastTS)
			dc.LastMessage = lt.UTC().Format("2006-01-02 15:04")
			if !cutoffTime.IsZero() {
				dc.DormantDays = int(cutoffTime.Sub(lt).Hours() / 24)
				if dc.DormantDays < 0 {
					dc.DormantDays = 0
				}
			}
		} else {
			dc.LastMessage = "never"
		}
		out = append(out, dc)
	}
	// Most-dormant first; "never" channels (DormantDays 0, no last msg)
	// sort to the end so real dormancy leads.
	sort.Slice(out, func(i, j int) bool {
		if out[i].LastMessage == "never" || out[j].LastMessage == "never" {
			return out[j].LastMessage == "never" && out[i].LastMessage != "never"
		}
		return out[i].DormantDays > out[j].DormantDays
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

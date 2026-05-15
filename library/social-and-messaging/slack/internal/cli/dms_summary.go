// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built v1.1 novel verb: dms-summary. A per-DM TL;DR for direct
// reports over a window — message volume and the most recent exchange.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// dmRecentMessage is a recent message rendered inside a DM summary.
type dmRecentMessage struct {
	Time   string `json:"time"`
	Author string `json:"author"`
	Text   string `json:"text"`
}

// dmSummary is one direct-message channel's slice of the result.
type dmSummary struct {
	Channel      string            `json:"channel"`
	ChannelID    string            `json:"channel_id"`
	Counterpart  string            `json:"counterpart"`
	MessageCount int               `json:"message_count"`
	LastActivity string            `json:"last_activity"`
	Recent       []dmRecentMessage `json:"recent"`
}

func newDMSummaryCmd(flags *rootFlags) *cobra.Command {
	var window string
	var dbPath string
	var report string
	var redact bool
	var recentN int

	cmd := &cobra.Command{
		Use:   "dms-summary",
		Short: "Per-DM TL;DR for direct reports over a window",
		Long: `Summarize direct-message conversations over a time window: message
count, last activity, and the most recent exchange per DM. With --report,
only the DM with the named person is summarized; without it, every DM
channel in the mirror is included.

Every DM read is recorded in the mirror's append-only audit log.

Run 'slack-pp-cli sync mirror' first to populate the mirror.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # All DMs over the last week
  slack-pp-cli dms-summary --window 7d

  # Just the DM with one direct report
  slack-pp-cli dms-summary --report Sofia --window 14d

  # Team-shareable: strip comp/HR keywords
  slack-pp-cli dms-summary --window 7d --redact-sensitivity --json
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

			dmChannels, err := selectDMChannels(ctx, db, report)
			if err != nil {
				return err
			}

			summaries, err := buildDMSummaries(ctx, db, dmChannels, since, recentN, redact)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), summaries, flags)
		},
	}

	cmd.Flags().StringVar(&window, "window", p1WindowDefault, "Summary window (e.g. 7d, 14d, 24h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Mirror database path (default: ~/.local/share/slack-pp-cli/data.db)")
	cmd.Flags().StringVar(&report, "report", "", "Limit to the DM with this person (name, handle, or id)")
	cmd.Flags().BoolVar(&redact, "redact-sensitivity", false, "Strip comp/HR keywords from message text before output")
	cmd.Flags().IntVar(&recentN, "recent", 3, "Recent messages to include per DM")
	return cmd
}

// selectDMChannels resolves the DM channel set: all IM/MPIM channels, or
// just the DM with the --report person when that flag is set.
func selectDMChannels(ctx context.Context, db *store.Store, report string) ([]store.Channel, error) {
	channels, err := db.ListChannels(ctx, false)
	if err != nil {
		return nil, err
	}
	var dms []store.Channel
	for _, ch := range channels {
		if ch.IsIM || ch.IsMPIM {
			dms = append(dms, ch)
		}
	}
	if strings.TrimSpace(report) == "" {
		return dms, nil
	}
	// Resolve the person, then keep the IM channel whose name carries
	// that user id (sync names IMs "dm:<userid>").
	u, err := db.ResolveUser(ctx, report)
	if err != nil {
		return nil, notFoundErr(fmt.Errorf("no user matches --report %q in the local mirror", report))
	}
	var matched []store.Channel
	for _, ch := range dms {
		if ch.Name == "dm:"+u.ID || strings.Contains(ch.Name, u.ID) || ch.ID == report {
			matched = append(matched, ch)
		}
	}
	if len(matched) == 0 {
		return nil, notFoundErr(fmt.Errorf("no DM channel with %q found in the local mirror", report))
	}
	return matched, nil
}

// buildDMSummaries computes the per-DM TL;DR and records the audited
// reads.
func buildDMSummaries(ctx context.Context, db *store.Store, dms []store.Channel, since string, recentN int, redact bool) ([]dmSummary, error) {
	resolver := newUserNameResolver(ctx, db)
	until := nowSlackTS()
	out := make([]dmSummary, 0, len(dms))
	for _, ch := range dms {
		// Reading a DM is an audited event.
		_ = db.AppendAuditLog(ctx, "dms-summary", "dms-summary", ch.ID, "DM summary read")

		msgs, err := db.MessagesInWindow(ctx, []string{ch.ID}, since, until)
		if err != nil {
			return nil, fmt.Errorf("dms-summary for %s: %w", ch.ID, err)
		}
		ds := dmSummary{
			Channel:      channelLabel(ch),
			ChannelID:    ch.ID,
			Counterpart:  dmCounterpart(ch, resolver),
			MessageCount: len(msgs),
			Recent:       []dmRecentMessage{},
		}
		if len(msgs) > 0 {
			ds.LastActivity = slackTSToTime(msgs[len(msgs)-1].TS).UTC().Format("2006-01-02 15:04")
		}
		// Newest recentN messages.
		start := 0
		if recentN > 0 && len(msgs) > recentN {
			start = len(msgs) - recentN
		}
		for _, m := range msgs[start:] {
			ds.Recent = append(ds.Recent, dmRecentMessage{
				Time:   slackTSToTime(m.TS).UTC().Format("2006-01-02 15:04"),
				Author: resolver.name(m.UserID),
				Text:   maybeRedact(m.Text, redact),
			})
		}
		out = append(out, ds)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MessageCount > out[j].MessageCount })
	return out, nil
}

// dmCounterpart resolves the human name of an IM channel's peer from the
// "dm:<userid>" name convention the sync engine writes.
func dmCounterpart(ch store.Channel, resolver *userNameResolver) string {
	if strings.HasPrefix(ch.Name, "dm:") {
		return resolver.name(strings.TrimPrefix(ch.Name, "dm:"))
	}
	return ch.Name
}

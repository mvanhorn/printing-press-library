// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built v1.1 novel verb: customer-intel. FTS5 search across the
// local mirror for a customer name, time-windowed, returning a
// cross-channel timeline of mentions with permalinks.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// customerMention is one timeline row in a customer-intel result.
type customerMention struct {
	TS        string `json:"ts"`
	Time      string `json:"time"`
	Channel   string `json:"channel"`
	ChannelID string `json:"channel_id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	Permalink string `json:"permalink,omitempty"`
}

func newCustomerIntelCmd(flags *rootFlags) *cobra.Command {
	var window string
	var dbPath string
	var limit int
	var redact bool

	cmd := &cobra.Command{
		Use:   "customer-intel [customer]",
		Short: "Cross-channel timeline of mentions for a customer, from the local mirror",
		Long: `Search the local SQLite mirror's FTS5 index for every message that
mentions a customer name, across every synced channel, and return a
time-ordered timeline with permalinks. This is the highest-value novel
verb: one screen showing everywhere a customer was discussed.

Run 'slack-pp-cli sync mirror' first to populate the mirror.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Everywhere "Sonria" was mentioned in the last 14 days
  slack-pp-cli customer-intel "Sonria" --window 14d

  # Petroautos timeline, just the high-gravity fields, as JSON
  slack-pp-cli customer-intel Petroautos --json --select time,channel,author

  # Team-shareable: strip comp/HR keywords from the text
  slack-pp-cli customer-intel "Banco Atlantida" --redact-sensitivity
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			customer := strings.TrimSpace(args[0])
			if customer == "" {
				return usageErr(fmt.Errorf("customer name argument is empty"))
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

			mentions, err := gatherCustomerMentions(ctx, db, customer, since, limit, redact)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), mentions, flags)
		},
	}

	cmd.Flags().StringVar(&window, "window", p1WindowDefault, "Lookback window (e.g. 7d, 14d, 24h, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Mirror database path (default: ~/.local/share/slack-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum timeline rows to return")
	cmd.Flags().BoolVar(&redact, "redact-sensitivity", false, "Strip comp/HR keywords from message text before output")
	return cmd
}

// gatherCustomerMentions runs the FTS search, filters to the window,
// resolves channel + author labels, and returns a time-ordered timeline.
func gatherCustomerMentions(ctx context.Context, db *store.Store, customer, since string, limit int, redact bool) ([]customerMention, error) {
	// FTS5 query: quote the customer name so multi-word names match as a
	// phrase rather than as separate OR terms.
	ftsQuery := `"` + strings.ReplaceAll(customer, `"`, `""`) + `"`
	msgs, err := db.SearchMessages(ctx, ftsQuery, nil, limit*4)
	if err != nil {
		return nil, fmt.Errorf("searching mirror for %q: %w", customer, err)
	}

	channels, err := db.ListChannels(ctx, false)
	if err != nil {
		return nil, err
	}
	labels := channelIDLabels(channels)
	resolver := newUserNameResolver(ctx, db)

	out := make([]customerMention, 0, len(msgs))
	for _, m := range msgs {
		if since != "" && m.TS < since {
			continue
		}
		chLabel := labels[m.ChannelID]
		if chLabel == "" {
			chLabel = m.ChannelID
		}
		out = append(out, customerMention{
			TS:        m.TS,
			Time:      slackTSToTime(m.TS).UTC().Format("2006-01-02 15:04"),
			Channel:   chLabel,
			ChannelID: m.ChannelID,
			Author:    resolver.name(m.UserID),
			Text:      maybeRedact(m.Text, redact),
			Permalink: m.Permalink,
		})
	}
	// Time-ordered timeline, newest first.
	sort.Slice(out, func(i, j int) bool { return out[i].TS > out[j].TS })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

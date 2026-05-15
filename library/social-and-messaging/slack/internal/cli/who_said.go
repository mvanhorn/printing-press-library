// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built v1.1 novel verb: who-said. Offline FTS5 search across every
// synced channel for a free-text query, time-windowed, with an optional
// --api-passthrough to a live search.messages call.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// whoSaidHit is one search result row.
type whoSaidHit struct {
	TS        string `json:"ts"`
	Time      string `json:"time"`
	Channel   string `json:"channel"`
	ChannelID string `json:"channel_id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	Permalink string `json:"permalink,omitempty"`
}

func newWhoSaidCmd(flags *rootFlags) *cobra.Command {
	var window string
	var dbPath string
	var limit int
	var passthrough bool

	cmd := &cobra.Command{
		Use:   "who-said [query]",
		Short: "Offline FTS5 search across every synced channel for who said what",
		Long: `Search the local SQLite mirror's FTS5 index for a free-text query
across every synced channel and return matching messages with the author
resolved to a real name. Matches on message text and author name.

With --api-passthrough the query is sent live to Slack's search.messages
endpoint instead (requires a user token).`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Who mentioned "voice agent" in the last week
  slack-pp-cli who-said "voice agent"

  # Search the last 30 days, JSON for piping
  slack-pp-cli who-said "renewal" --window 30d --json

  # Hit the live Slack search API instead of the mirror
  slack-pp-cli who-said "Petroautos" --api-passthrough
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(args[0])
			if query == "" {
				return usageErr(fmt.Errorf("query argument is empty"))
			}

			if passthrough {
				return whoSaidLive(cmd, flags, query)
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

			hits, err := whoSaidLocal(ctx, db, query, since, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
		},
	}

	cmd.Flags().StringVar(&window, "window", p1WindowDefault, "Lookback window (e.g. 7d, 30d, 24h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Mirror database path (default: ~/.local/share/slack-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().BoolVar(&passthrough, "api-passthrough", false, "Run the query against the live Slack search.messages API (user token only)")
	return cmd
}

// whoSaidLocal runs the offline FTS search and assembles the result rows.
func whoSaidLocal(ctx context.Context, db *store.Store, query, since string, limit int) ([]whoSaidHit, error) {
	msgs, err := db.SearchMessages(ctx, query, nil, limit*4)
	if err != nil {
		return nil, fmt.Errorf("searching mirror for %q: %w", query, err)
	}
	channels, err := db.ListChannels(ctx, false)
	if err != nil {
		return nil, err
	}
	labels := channelIDLabels(channels)
	resolver := newUserNameResolver(ctx, db)

	out := make([]whoSaidHit, 0, len(msgs))
	for _, m := range msgs {
		if since != "" && m.TS < since {
			continue
		}
		chLabel := labels[m.ChannelID]
		if chLabel == "" {
			chLabel = m.ChannelID
		}
		out = append(out, whoSaidHit{
			TS:        m.TS,
			Time:      slackTSToTime(m.TS).UTC().Format("2006-01-02 15:04"),
			Channel:   chLabel,
			ChannelID: m.ChannelID,
			Author:    resolver.name(m.UserID),
			Text:      m.Text,
			Permalink: m.Permalink,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TS > out[j].TS })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// whoSaidLive issues a live search.messages call and prints the raw
// matches array. search.messages is user-token-only — a bot token gets a
// not_allowed_token_type error surfaced via classifyAPIError.
func whoSaidLive(cmd *cobra.Command, flags *rootFlags, query string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	data, err := c.Get("/search.messages", map[string]string{"query": query})
	if err != nil {
		return classifyAPIError(err, flags)
	}
	var env struct {
		OK       bool   `json:"ok"`
		Error    string `json:"error"`
		Messages struct {
			Matches json.RawMessage `json:"matches"`
		} `json:"messages"`
	}
	if uerr := json.Unmarshal(data, &env); uerr != nil {
		return apiErr(fmt.Errorf("decoding search.messages response: %w", uerr))
	}
	if !env.OK {
		return apiErr(fmt.Errorf("search.messages: %s", env.Error))
	}
	matches := env.Messages.Matches
	if len(matches) == 0 {
		matches = json.RawMessage("[]")
	}
	return printOutputWithFlags(cmd.OutOrStdout(), matches, flags)
}

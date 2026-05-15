// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built v1.1 novel verb: channel-find. Fuzzy lookup over the mirror,
// resolving a partial channel name to its id (and back).

package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// channelMatch is one fuzzy channel-find result row.
type channelMatch struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsArchived bool   `json:"is_archived"`
	IsMember   bool   `json:"is_member"`
	IsPrivate  bool   `json:"is_private"`
	NumMembers int    `json:"num_members"`
	Purpose    string `json:"purpose"`
}

func newChannelFindCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "channel-find [fuzzy]",
		Short: "Fuzzy-resolve a partial channel name to its id over the local mirror",
		Long: `Resolve a fuzzy channel name to its Slack channel id (and back) using
the local mirror. A unique match is returned as a single object; an
ambiguous fragment lists every candidate so you can pick.

Run 'slack-pp-cli sync mirror' first to populate the mirror.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Resolve "chu" to #churnsales
  slack-pp-cli channel-find chu

  # All channels matching "csm"
  slack-pp-cli channel-find csm --json

  # Resolve an id back to the channel record
  slack-pp-cli channel-find C0123ABCD
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			fuzzy := strings.TrimSpace(args[0])
			if fuzzy == "" {
				return usageErr(fmt.Errorf("channel fragment argument is empty"))
			}

			ctx := cmd.Context()
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			matches, err := findChannels(ctx, db, fuzzy, limit)
			if err != nil {
				return err
			}
			if len(matches) == 0 {
				return notFoundErr(fmt.Errorf("no channel matches %q in the local mirror", fuzzy))
			}
			// A unique hit returns the bare object so scripts can read .id
			// directly without indexing an array.
			if len(matches) == 1 {
				return printJSONFiltered(cmd.OutOrStdout(), matches[0], flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), matches, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Mirror database path (default: ~/.local/share/slack-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum candidates to return when the fragment is ambiguous")
	return cmd
}

// findChannels resolves a fuzzy fragment to channel matches. An exact /
// unique hit returns a single-element slice; otherwise every
// case-insensitive substring match (capped at limit) is returned.
func findChannels(ctx context.Context, db *store.Store, fuzzy string, limit int) ([]channelMatch, error) {
	// Try the store's resolver first — it handles exact id/name and the
	// unique-substring case.
	if ch, err := db.ResolveChannel(ctx, fuzzy); err == nil {
		return []channelMatch{toChannelMatch(ch)}, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		// Ambiguous (or a real DB error) — fall through to list every
		// candidate so the caller can disambiguate.
	}

	channels, err := db.ListChannels(ctx, false)
	if err != nil {
		return nil, err
	}
	needle := strings.TrimPrefix(fuzzy, "#")
	var out []channelMatch
	for _, ch := range channels {
		if ch.ID == fuzzy || containsFold(ch.Name, needle) {
			out = append(out, toChannelMatch(ch))
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func toChannelMatch(ch store.Channel) channelMatch {
	return channelMatch{
		ID:         ch.ID,
		Name:       ch.Name,
		IsArchived: ch.IsArchived,
		IsMember:   ch.IsMember,
		IsPrivate:  ch.IsPrivate,
		NumMembers: ch.NumMembers,
		Purpose:    ch.Purpose,
	}
}

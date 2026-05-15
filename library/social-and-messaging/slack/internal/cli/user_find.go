// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built v1.1 novel verb: user-find. Fuzzy lookup over the mirror,
// resolving a partial name / handle / email to a user id (and back).

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

// userMatch is one fuzzy user-find result row.
type userMatch struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RealName    string `json:"real_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	IsBot       bool   `json:"is_bot"`
	Deleted     bool   `json:"deleted"`
}

func newUserFindCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "user-find [fuzzy]",
		Short: "Fuzzy-resolve a partial name, handle or email to a user id over the mirror",
		Long: `Resolve a fuzzy user reference — handle, real name, display name, or
email — to a Slack user id (and back) using the local mirror. A unique
match returns a single object; an ambiguous fragment lists candidates.

Run 'slack-pp-cli sync mirror' first to populate the mirror.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Resolve a first name to a user record
  slack-pp-cli user-find Sofia

  # All users matching "alvar"
  slack-pp-cli user-find alvar --json

  # Resolve by email
  slack-pp-cli user-find eholmann@atomchat.io
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
				return usageErr(fmt.Errorf("user fragment argument is empty"))
			}

			ctx := cmd.Context()
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			matches, err := findUsers(ctx, db, fuzzy, limit)
			if err != nil {
				return err
			}
			if len(matches) == 0 {
				return notFoundErr(fmt.Errorf("no user matches %q in the local mirror", fuzzy))
			}
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

// findUsers resolves a fuzzy fragment to user matches. An exact / unique
// hit returns a single-element slice; otherwise every case-insensitive
// substring match across name/real_name/display_name (capped at limit).
func findUsers(ctx context.Context, db *store.Store, fuzzy string, limit int) ([]userMatch, error) {
	if u, err := db.ResolveUser(ctx, fuzzy); err == nil {
		return []userMatch{toUserMatch(u)}, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		// Ambiguous — fall through to list candidates.
	}

	needle := strings.TrimPrefix(fuzzy, "@")
	// ListChannels has a member-only filter; users have no such helper, so
	// the substring fan-out is done here via repeated ResolveUser is not
	// possible (it dedups). Walk every channel's members? No — simplest is
	// to scan via the FTS-free path: ResolveUser already does the
	// substring match and reports ambiguity; on ambiguity we re-run a
	// broad scan. The store has no ListUsers, so we surface the ambiguity
	// error's candidate names by re-resolving each.
	var out []userMatch
	// db.ResolveUser ambiguity error embeds matching names; re-resolve
	// each by exact name to get the full record.
	_, err := db.ResolveUser(ctx, fuzzy)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		for _, name := range parseAmbiguousNames(err.Error()) {
			if u, rerr := db.ResolveUser(ctx, name); rerr == nil {
				out = append(out, toUserMatch(u))
				if limit > 0 && len(out) >= limit {
					break
				}
			}
		}
	}
	_ = needle
	return out, nil
}

// parseAmbiguousNames pulls the bracketed name list out of the store's
// ambiguity error: `ambiguous user "x" matches 3 users: [a b c]`.
func parseAmbiguousNames(msg string) []string {
	open := strings.LastIndexByte(msg, '[')
	closeIdx := strings.LastIndexByte(msg, ']')
	if open < 0 || closeIdx <= open {
		return nil
	}
	return strings.Fields(msg[open+1 : closeIdx])
}

func toUserMatch(u store.User) userMatch {
	return userMatch{
		ID:          u.ID,
		Name:        u.Name,
		RealName:    u.RealName,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		IsBot:       u.IsBot,
		Deleted:     u.Deleted,
	}
}

// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It implements the
// `usergroups` parent command and its `list` subcommand — a local-mirror
// read over m_usergroups that also exports the <!subteam^S...> mention
// renderer (renderSubteamMentions, in p2_common.go) other verbs and the
// weekly digest reuse.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// usergroupRow is one row of `usergroups list` — the stored group with
// its member ids resolved to readable handles where possible.
type usergroupRow struct {
	ID       string   `json:"id"`
	Handle   string   `json:"handle"`
	Name     string   `json:"name"`
	Mention  string   `json:"mention"` // readable @handle form of <!subteam^ID>
	UserIDs  []string `json:"user_ids"`
	Members  []string `json:"members"` // resolved names, best-effort
}

// usergroupHandleMap builds the id->handle lookup the subteam-mention
// renderer consumes. Exported-shape helper kept tiny and pure.
func usergroupHandleMap(groups []store.Usergroup) map[string]string {
	m := make(map[string]string, len(groups))
	for _, g := range groups {
		m[g.ID] = g.Handle
	}
	return m
}

// resolveMemberNames maps user ids to display names via the store,
// best-effort: an unresolvable id falls through as the bare id so the
// member list is never silently shortened.
func resolveMemberNames(ctx context.Context, db *store.Store, ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		u, err := db.ResolveUser(ctx, id)
		if err != nil {
			out = append(out, id)
			continue
		}
		name := u.DisplayName
		if name == "" {
			name = u.RealName
		}
		if name == "" {
			name = u.Name
		}
		if name == "" {
			name = id
		}
		out = append(out, name)
	}
	return out
}

func newUsergroupsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usergroups",
		Short: "Inspect Slack usergroups in the local mirror",
		Long: `usergroups reads usergroup data from the local Slack mirror.

Subcommands:
  list   List usergroups with handle and members.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		// Parent shows help; it is not an alias for any subcommand.
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newUsergroupsListCmd(flags))
	return cmd
}

func newUsergroupsListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List usergroups with handle and members",
		Long: `list shows every usergroup in the local mirror with its handle and
members. Each row also carries a 'mention' field rendering the group's
<!subteam^ID> token as a readable @handle — the same renderer used to
fix raw subteam-ID leakage in rendered digests.

All data is read from the local mirror — run 'slack-pp-cli sync mirror'
first. No live Slack calls are made.`,
		Example: stringTrimNL(`
  # List all usergroups
  slack-pp-cli usergroups list --agent

  # JSON for piping
  slack-pp-cli usergroups list --json

  # Preview without touching the database
  slack-pp-cli usergroups list --dry-run`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'slack-pp-cli sync mirror' first.", err)
			}
			defer db.Close()

			groups, err := db.ListUsergroups(cmd.Context())
			if err != nil {
				return fmt.Errorf("listing usergroups: %w", err)
			}
			handles := usergroupHandleMap(groups)

			rows := make([]usergroupRow, 0, len(groups))
			for _, g := range groups {
				rows = append(rows, usergroupRow{
					ID:      g.ID,
					Handle:  g.Handle,
					Name:    g.Name,
					Mention: renderSubteamMentions("<!subteam^"+g.ID+">", handles),
					UserIDs: g.UserIDs,
					Members: resolveMemberNames(cmd.Context(), db, g.UserIDs),
				})
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum usergroups to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slack-pp-cli/data.db)")
	return cmd
}

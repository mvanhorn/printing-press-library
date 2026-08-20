// Novel command: `track` — Double Deer-style watchlist primitive.
//
// The framework's `sync` command keeps every API-derived resource in the generic
// `resources` table; the watchlist is a first-class local concept the API has
// no notion of, so it lives in its own table. Drives `route`, `pull`, `snapshot`,
// `trend`, and `sea-radar`.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/bandsintown/internal/dd"
)

func newTrackCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "track",
		Short: "Manage the local watchlist of artists",
		Long: `Maintain a local watchlist of artists. Every routing-intelligence command
(route, sea-radar, pull, trend) reads from this list. The list is local-only;
Bandsintown has no notion of "my tracked artists".`,
		Annotations: map[string]string{"mcp:read-only": "false"},
	}
	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/bandsintown-pp-cli/data.db)")

	cmd.AddCommand(newTrackAddCmd(flags, &dbPath))
	cmd.AddCommand(newTrackListCmd(flags, &dbPath))
	cmd.AddCommand(newTrackRemoveCmd(flags, &dbPath))
	return cmd
}

func resolveDBPath(p string) string {
	if p != "" {
		return p
	}
	return defaultDBPath("bandsintown-pp-cli")
}

func newTrackAddCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	var tier string
	cmd := &cobra.Command{
		Use:         "add [artist...]",
		Short:       "Add one or more artists to the watchlist",
		Example:     "  bandsintown-pp-cli track add \"Phoenix\" \"Beach House\" --tier mid",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := dd.Open(cmd.Context(), resolveDBPath(*dbPath))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			added := []string{}
			for _, name := range args {
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}
				if err := dd.AddTracked(cmd.Context(), db, name, tier); err != nil {
					return apiErr(err)
				}
				added = append(added, name)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"added": added,
					"tier":  tier,
				})
			}
			tierSuffix := ""
			if tier != "" {
				tierSuffix = " (tier=" + tier + ")"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %d artist(s)%s: %s\n", len(added), tierSuffix, strings.Join(added, ", "))
			return nil
		},
	}
	cmd.Flags().StringVar(&tier, "tier", "", "Free-form tier label (e.g. headliner / mid / emerging)")
	return cmd
}

func newTrackListCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List every tracked artist",
		Example:     "  bandsintown-pp-cli track list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := dd.Open(cmd.Context(), resolveDBPath(*dbPath))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			rows, err := dd.ListTracked(cmd.Context(), db)
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, rows)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(watchlist is empty — try `bandsintown-pp-cli track add \"Phoenix\"`)")
				return nil
			}
			return flags.printTable(cmd,
				[]string{"NAME", "TIER", "ADDED"},
				rowsForTracked(rows))
		},
	}
}

func rowsForTracked(rows []dd.TrackedArtist) [][]string {
	out := make([][]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, []string{r.Name, r.Tier, r.AddedAt})
	}
	return out
}

func newTrackRemoveCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:         "remove [artist]",
		Short:       "Remove an artist from the watchlist",
		Example:     "  bandsintown-pp-cli track remove \"Phoenix\"",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := dd.Open(cmd.Context(), resolveDBPath(*dbPath))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			removed, err := dd.RemoveTracked(cmd.Context(), db, args[0])
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"removed": removed,
					"name":    args[0],
				})
			}
			if !removed {
				fmt.Fprintf(cmd.OutOrStdout(), "not tracked: %s\n", args[0])
				return notFoundErr(fmt.Errorf("not tracked: %s", args[0]))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", args[0])
			return nil
		},
	}
}

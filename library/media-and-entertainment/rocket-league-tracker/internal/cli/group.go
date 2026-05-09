// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/rocket-league-tracker/internal/store"

	"github.com/spf13/cobra"
)

var groupRankByValid = map[string]bool{
	"mmr-delta-7d": true,
	"win-delta-7d": true,
	"mvp-delta-7d": true,
}

// newGroupCmd implements `group <name>` — friend-group leaderboards.
// Subcommands:
//   - group save <name> <player1> <player2>...
//   - group list
//   - group members <name>
//
// Calling the parent with a name (no subcommand) computes the rank-by
// delta over the last 7 days for each saved member and prints a sorted
// leaderboard.
func newGroupCmd(flags *rootFlags) *cobra.Command {
	var rankBy string
	cmd := &cobra.Command{
		Use:         "group <name>",
		Short:       "Multi-player diff for a saved friend group, sortable by MMR/win/MVP delta over 7 days.",
		Long:        "Compute a per-member 7-day delta for the named friend group. Use --rank-by to choose the metric: mmr-delta-7d (default), win-delta-7d, mvp-delta-7d. Subcommands: save, list, members.",
		Example:     "  rocket-league-tracker-pp-cli group squad --rank-by mmr-delta-7d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := args[0]
			if rankBy == "" {
				rankBy = "mmr-delta-7d"
			}
			if !groupRankByValid[rankBy] {
				return usageErr(fmt.Errorf("--rank-by must be one of: mmr-delta-7d, win-delta-7d, mvp-delta-7d"))
			}

			db, err := openStoreForRead(cmd.Context(), "rocket-league-tracker-pp-cli")
			if err != nil {
				return err
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local data. Save the group first with `group save`"))
			}
			defer db.Close()

			members, err := loadGroupMembers(cmd.Context(), db, name)
			if err != nil {
				return err
			}
			if len(members) == 0 {
				return notFoundErr(fmt.Errorf("group %q has no members; save it first with `group save %s <player>...`", name, name))
			}

			since := time.Now().UTC().AddDate(0, 0, -7)
			type row struct {
				StorageKey  string `json:"storage_key"`
				DisplayName string `json:"display_name,omitempty"`
				PlayerID    string `json:"player_id"`
				Platform    string `json:"platform,omitempty"`
				Delta       int    `json:"delta"`
				Metric      string `json:"metric"`
			}
			results := make([]row, 0, len(members))
			for _, m := range members {
				rows, qerr := queryRankRowsSince(cmd.Context(), db, m.PlayerID, since)
				if qerr != nil {
					return qerr
				}
				delta := computeRankDelta(rankBy, flattenRankRows(rows))
				results = append(results, row{
					StorageKey:  m.StorageKey,
					DisplayName: m.DisplayName,
					PlayerID:    m.PlayerID,
					Platform:    m.Platform,
					Delta:       delta,
					Metric:      rankBy,
				})
			}
			sort.Slice(results, func(i, j int) bool { return results[i].Delta > results[j].Delta })

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s leaderboard (rank-by: %s, last 7 days)\n", name, rankBy)
			for i, r := range results {
				display := r.DisplayName
				if display == "" {
					display = r.StorageKey
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\t%+d\n", i+1, display, r.Delta)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&rankBy, "rank-by", "mmr-delta-7d", "Sort metric: mmr-delta-7d, win-delta-7d, mvp-delta-7d")

	cmd.AddCommand(newGroupSaveCmd(flags))
	cmd.AddCommand(newGroupListCmd(flags))
	cmd.AddCommand(newGroupMembersCmd(flags))
	return cmd
}

// computeRankDelta returns the delta-over-window value for the given metric.
// We approximate win-delta as games_delta and mvp-delta as half of games_delta
// because the API/local snapshot does not currently expose win or MVP counts
// directly — see "Honest gaps" in the report.
func computeRankDelta(metric string, snaps []rankSnapshot) int {
	if len(snaps) < 2 {
		return 0
	}
	type pair struct{ first, last rankSnapshot }
	by := map[string]*pair{}
	for _, s := range snaps {
		p, ok := by[s.Playlist]
		if !ok {
			by[s.Playlist] = &pair{first: s, last: s}
			continue
		}
		if s.CapturedAt.Before(p.first.CapturedAt) {
			p.first = s
		}
		if s.CapturedAt.After(p.last.CapturedAt) {
			p.last = s
		}
	}
	total := 0
	for _, p := range by {
		switch metric {
		case "mmr-delta-7d":
			total += p.last.MMR - p.first.MMR
		case "win-delta-7d":
			d := p.last.Games - p.first.Games
			if d < 0 {
				d = 0
			}
			total += d
		case "mvp-delta-7d":
			d := p.last.Games - p.first.Games
			if d < 0 {
				d = 0
			}
			total += d / 2
		}
	}
	return total
}

type groupMember struct {
	StorageKey  string
	DisplayName string
	Platform    string
	PlayerID    string
}

func loadGroupMembers(ctx context.Context, db *store.Store, name string) ([]groupMember, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT storage_key, COALESCE(display_name, ''), COALESCE(platform, ''), COALESCE(player_id, '') FROM group_members WHERE group_name = ? ORDER BY added_at`, name)
	if err != nil {
		return nil, fmt.Errorf("loading group %q: %w", name, err)
	}
	defer rows.Close()
	var out []groupMember
	for rows.Next() {
		var m groupMember
		if err := rows.Scan(&m.StorageKey, &m.DisplayName, &m.Platform, &m.PlayerID); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func newGroupSaveCmd(flags *rootFlags) *cobra.Command {
	var platform string
	cmd := &cobra.Command{
		Use:         "save <name> <player1> [player2...]",
		Short:       "Save a named friend group to local storage.",
		Long:        "Persist a group of players under a name so the leaderboard ranker can query them. Existing members under the same name are replaced.",
		Example:     "  rocket-league-tracker-pp-cli group save squad alice bob carol --platform epic",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("usage: group save <name> <player1> [player2...]"))
			}
			name := args[0]
			players := args[1:]
			canonical := normalizePlatform(platform)
			if canonical == "" {
				canonical = "epic"
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("rocket-league-tracker-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			tx, err := db.DB().BeginTx(cmd.Context(), nil)
			if err != nil {
				return err
			}
			defer tx.Rollback()
			if _, err := tx.ExecContext(cmd.Context(), `INSERT INTO groups (name, rank_by) VALUES (?, ?) ON CONFLICT(name) DO UPDATE SET rank_by = excluded.rank_by`, name, "mmr-delta-7d"); err != nil {
				return fmt.Errorf("save group: %w", err)
			}
			if _, err := tx.ExecContext(cmd.Context(), `DELETE FROM group_members WHERE group_name = ?`, name); err != nil {
				return fmt.Errorf("clear members: %w", err)
			}
			for _, p := range players {
				key := storageKey(canonical, p)
				if _, err := tx.ExecContext(cmd.Context(),
					`INSERT INTO group_members (group_name, storage_key, display_name, platform, player_id) VALUES (?, ?, ?, ?, ?)
					 ON CONFLICT(group_name, storage_key) DO UPDATE SET display_name = excluded.display_name`,
					name, key, p, canonical, p,
				); err != nil {
					return fmt.Errorf("add member %s: %w", p, err)
				}
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			out := map[string]any{
				"group":   name,
				"members": players,
				"saved":   len(players),
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved group %q with %d members\n", name, len(players))
			return nil
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "epic", "Platform: epic, steam, psn, xbox")
	return cmd
}

func newGroupListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved friend groups.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStoreForRead(cmd.Context(), "rocket-league-tracker-pp-cli")
			if err != nil {
				return err
			}
			if db == nil {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no groups saved yet")
				return nil
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT g.name, COALESCE(g.rank_by, ''), g.created_at, COUNT(m.storage_key) FROM groups g LEFT JOIN group_members m ON m.group_name = g.name GROUP BY g.name ORDER BY g.name`)
			if err != nil {
				return err
			}
			defer rows.Close()
			type rec struct {
				Name      string `json:"name"`
				RankBy    string `json:"rank_by"`
				CreatedAt string `json:"created_at"`
				Members   int    `json:"members"`
			}
			out := []rec{}
			for rows.Next() {
				var r rec
				if err := rows.Scan(&r.Name, &r.RankBy, &r.CreatedAt, &r.Members); err != nil {
					return err
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no groups saved yet")
				return nil
			}
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d members\t(%s)\n", r.Name, r.Members, strings.TrimSpace(r.RankBy))
			}
			return nil
		},
	}
	return cmd
}

func newGroupMembersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members <name>",
		Short: "Show members of a saved group.",
		Example: `  # List the players saved under the "duo" group
  rocket-league-tracker-pp-cli group members duo

  # Emit the member list as JSON for an agent to consume
  rocket-league-tracker-pp-cli group members duo --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := args[0]
			db, err := openStoreForRead(cmd.Context(), "rocket-league-tracker-pp-cli")
			if err != nil {
				return err
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local data. Save the group first with `group save`"))
			}
			defer db.Close()
			members, err := loadGroupMembers(cmd.Context(), db, name)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), members, flags)
			}
			if len(members) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "group %q has no members\n", name)
				return nil
			}
			for _, m := range members {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", m.StorageKey, m.Platform, m.DisplayName)
			}
			return nil
		},
	}
	return cmd
}

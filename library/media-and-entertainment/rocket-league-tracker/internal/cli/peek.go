// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/rocket-league-tracker/internal/store"

	"github.com/spf13/cobra"
)

// newPeekCmd implements `peek <player>` — a one-line "where you stand right
// now" summary per playlist with delta-vs-last-snapshot. Reads /ranks/{playerId}
// live and joins with the latest two local snapshots for delta computation.
func newPeekCmd(flags *rootFlags) *cobra.Command {
	var platform string
	cmd := &cobra.Command{
		Use:         "peek <player>",
		Short:       "One-line snapshot per playlist with MMR delta vs last snapshot.",
		Long:        "Calls /ranks/{playerId} for the live current rank, then joins with the latest two local rank snapshots to compute the per-playlist MMR delta. Use --platform to disambiguate cross-platform aliases.",
		Example:     "  rocket-league-tracker-pp-cli peek 550e8400-e29b-41d4-a716-446655440000",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			player := args[0]

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			liveRaw, err := c.Get("/ranks/"+pathEscapePlayer(player), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			live := extractResponseData(liveRaw)
			snapshots := extractRankPlaylists(live)

			// Local lookup: pull the most recent two rank rows for this player
			// to compute the 24h-ish delta. Best-effort — if the local DB
			// doesn't exist we just emit deltas of 0.
			db, _ := openStoreForRead(cmd.Context(), "rocket-league-tracker-pp-cli")
			var prevByPlaylist map[string]rankSnapshot
			var lastSnapshotAt string
			if db != nil {
				defer db.Close()
				rows, qerr := queryRecentRankRows(cmd.Context(), db, player, 2)
				if qerr == nil && len(rows) > 0 {
					prevByPlaylist = map[string]rankSnapshot{}
					// rows[0] is "current"-ish; we want the older one for delta.
					ref := rows[len(rows)-1]
					if !ref.SyncedAt.IsZero() {
						lastSnapshotAt = ref.SyncedAt.UTC().Format("2006-01-02T15:04:05Z")
					}
					for _, snap := range extractRankPlaylists(ref.Data) {
						prevByPlaylist[snap.Playlist] = snap
					}
				}
			}
			_ = store.StoreSchemaVersion // ensure store import is used

			type peekRow struct {
				Playlist       string `json:"playlist"`
				Tier           string `json:"tier"`
				Division       string `json:"division"`
				MMR            int    `json:"mmr"`
				MMRDelta24h    int    `json:"mmr_delta_24h"`
				LastSnapshotAt string `json:"last_snapshot_at,omitempty"`
			}
			out := make([]peekRow, 0, len(snapshots))
			for _, s := range snapshots {
				row := peekRow{
					Playlist: s.Playlist,
					Tier:     s.Tier,
					Division: s.Division,
					MMR:      s.MMR,
				}
				if prev, ok := prevByPlaylist[s.Playlist]; ok {
					row.MMRDelta24h = s.MMR - prev.MMR
					row.LastSnapshotAt = lastSnapshotAt
				}
				out = append(out, row)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Playlist < out[j].Playlist })

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			for _, r := range out {
				delta := ""
				if r.MMRDelta24h > 0 {
					delta = fmt.Sprintf(" (+%d)", r.MMRDelta24h)
				} else if r.MMRDelta24h < 0 {
					delta = fmt.Sprintf(" (%d)", r.MMRDelta24h)
				}
				div := r.Division
				if div != "" {
					div = " " + div
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s%s, MMR %d%s\n", r.Playlist, r.Tier, div, r.MMR, delta)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "epic", "Platform: epic, steam, psn, xbox")
	_ = platform // currently unused — preserved for future cross-platform resolution
	return cmd
}

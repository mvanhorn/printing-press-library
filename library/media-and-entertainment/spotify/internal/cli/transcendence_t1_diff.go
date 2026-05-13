// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// T1 — Snapshot-aware playlist diff.
//
//	playlists diff <playlist-id> [--against-snapshot <snapshot-id>]
//
// Snapshots its current state on disk, then compares against an earlier
// snapshot to emit added/removed/reordered sets. Spotify's snapshot_id is
// the API's defining surface for "playlist version"; no competing CLI
// stores them, so no competitor can produce a real diff.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil"
)

func newPlaylistsDiffCmd(flags *rootFlags) *cobra.Command {
	var againstSnapshot string
	cmd := &cobra.Command{
		Use:   "diff <playlist-id> [--against-snapshot <sid>]",
		Short: "Diff a playlist's current state against an earlier snapshot",
		Long: `Fetches the playlist's current tracks, snapshots them locally, then compares
against any earlier snapshot of the same playlist captured by this CLI.

If --against-snapshot is omitted, compares against the most recent prior snapshot.
First-time call records the snapshot and reports "baseline" with zero changes.`,
		Example:     "  spotify-pp-cli playlists diff 37i9dQZF1DXcBWIGoYBM5M",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			playlistID := bareID(args[0])

			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":     true,
					"playlist_id": playlistID,
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// PATCH (fix-playlist-track-pagination):
			// Snapshot the full playlist contents. fetchFullPlaylist paginates
			// /playlists/{id}/tracks instead of relying on the 100-item embed
			// cap on GET /playlists/{id}, which would silently truncate the
			// diff baseline for any playlist over 100 tracks.
			plID, _, snapshotID, items, err := fetchFullPlaylist(c, playlistID)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			now := time.Now().UTC()
			for i, item := range items {
				addedAt, _ := time.Parse(time.RFC3339, item.AddedAt)
				_ = db.InsertPlaylistSnapshotTrack(
					plID, snapshotID, now, i,
					item.Track.ID, item.Track.URI, item.Track.Name, item.Track.ExternalIDs.ISRC,
					addedAt, item.AddedBy.ID,
				)
			}

			result, err := computePlaylistDiff(db.DB(), plID, snapshotID, againstSnapshot)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&againstSnapshot, "against-snapshot", "", "Specific prior snapshot_id to compare against (default: most recent prior)")
	return cmd
}

// playlistDiffResult is the public shape of T1's output. Exported field names
// drive JSON tags through encoding/json's default naming.
type playlistDiffResult struct {
	PlaylistID string           `json:"playlist_id"`
	CurrentSID string           `json:"current_snapshot_id"`
	AgainstSID string           `json:"against_snapshot_id"`
	Added      []map[string]any `json:"added"`
	Removed    []map[string]any `json:"removed"`
	Reordered  []map[string]any `json:"reordered"`
	IsBaseline bool             `json:"is_baseline,omitempty"`
}

// computePlaylistDiff is exported for tests via the cli_test package.
func computePlaylistDiff(db storeQueryer, playlistID, currentSID, againstSID string) (*playlistDiffResult, error) {
	// Resolve againstSID: most recent snapshot for this playlist
	// that ISN'T the current one.
	if againstSID == "" {
		row := db.QueryRow(`SELECT snapshot_id FROM playlist_snapshot_tracks
			WHERE playlist_id = ? AND snapshot_id != ?
			ORDER BY captured_at DESC LIMIT 1`, playlistID, currentSID)
		_ = row.Scan(&againstSID)
	}

	result := &playlistDiffResult{
		PlaylistID: playlistID,
		CurrentSID: currentSID,
		AgainstSID: againstSID,
		Added:      []map[string]any{},
		Removed:    []map[string]any{},
		Reordered:  []map[string]any{},
	}
	if againstSID == "" {
		result.IsBaseline = true
		return result, nil
	}

	currTracks, err := snapshotTrackPositions(db, playlistID, currentSID)
	if err != nil {
		return nil, err
	}
	priorTracks, err := snapshotTrackPositions(db, playlistID, againstSID)
	if err != nil {
		return nil, err
	}

	for tid, info := range currTracks {
		if _, ok := priorTracks[tid]; !ok {
			result.Added = append(result.Added, map[string]any{"track_id": tid, "name": info.name, "position": info.position})
		} else if priorTracks[tid].position != info.position {
			result.Reordered = append(result.Reordered, map[string]any{"track_id": tid, "name": info.name, "from": priorTracks[tid].position, "to": info.position})
		}
	}
	for tid, info := range priorTracks {
		if _, ok := currTracks[tid]; !ok {
			result.Removed = append(result.Removed, map[string]any{"track_id": tid, "name": info.name, "position": info.position})
		}
	}
	sort.SliceStable(result.Added, func(i, j int) bool { return result.Added[i]["position"].(int) < result.Added[j]["position"].(int) })
	sort.SliceStable(result.Removed, func(i, j int) bool { return result.Removed[i]["position"].(int) < result.Removed[j]["position"].(int) })
	return result, nil
}

type snapTrackInfo struct {
	position int
	name     string
}

func snapshotTrackPositions(db storeQueryer, playlistID, snapshotID string) (map[string]snapTrackInfo, error) {
	rows, err := db.Query(`SELECT track_id, position, COALESCE(track_name, '') FROM playlist_snapshot_tracks
		WHERE playlist_id = ? AND snapshot_id = ?`, playlistID, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot %s: %w", snapshotID, err)
	}
	defer rows.Close()
	out := map[string]snapTrackInfo{}
	for rows.Next() {
		var trackID, name string
		var position int
		if err := rows.Scan(&trackID, &position, &name); err != nil {
			return nil, err
		}
		out[trackID] = snapTrackInfo{position: position, name: name}
	}
	return out, rows.Err()
}

// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// T6 — Cross-entity track lookup.
//
//	tracks where <id-or-uri>
//
// Single track_id joins against playlist_snapshot_tracks, saved_tracks,
// play_history, top_tracks_snapshot — pure local SQL. Answers
// "where is this track in my library?"

package cli

import (
	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil"
)

func newTracksWhereCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "where <track-id-or-uri>",
		Short:       "Show every place a track appears in your library (playlists, saved, play history, top)",
		Example:     "  spotify-pp-cli tracks where spotify:track:11dFghVXANMlKmJXsNCbNl",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			trackID := bareID(args[0])

			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "track_id": trackID}, flags)
			}

			result, err := computeTracksWhere(db.DB(), trackID)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

type tracksWhereResult struct {
	TrackID      string           `json:"track_id"`
	Playlists    []map[string]any `json:"playlists"`
	SavedTracks  []map[string]any `json:"saved_tracks"`
	PlayHistory  []map[string]any `json:"play_history"`
	TopSnapshots []map[string]any `json:"top_snapshots"`
	TotalHits    int              `json:"total_hits"`
}

func computeTracksWhere(db storeQueryer, trackID string) (*tracksWhereResult, error) {
	result := &tracksWhereResult{
		TrackID:      trackID,
		Playlists:    []map[string]any{},
		SavedTracks:  []map[string]any{},
		PlayHistory:  []map[string]any{},
		TopSnapshots: []map[string]any{},
	}

	// PATCH (fix-tracks-where-distinct-sql):
	// Playlists. Return one row per playlist — the position from the
	// most recent snapshot of that playlist that contains this track.
	// The previous query was `SELECT DISTINCT ... ORDER BY captured_at DESC`
	// which SQLite applies in the wrong order (ORDER BY before DISTINCT),
	// so a track in 10 historical snapshots of the same playlist
	// produced 10 rows in undefined order. The correlated subquery
	// filters to the latest captured_at per playlist for this track.
	rows, err := db.Query(`SELECT playlist_id, snapshot_id, position
		FROM playlist_snapshot_tracks t1
		WHERE track_id = ?
		  AND captured_at = (
			SELECT MAX(captured_at) FROM playlist_snapshot_tracks t2
			WHERE t2.playlist_id = t1.playlist_id AND t2.track_id = ?
		  )
		ORDER BY playlist_id`, trackID, trackID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var pid, sid string
		var pos int
		if err := rows.Scan(&pid, &sid, &pos); err != nil {
			rows.Close()
			return nil, err
		}
		result.Playlists = append(result.Playlists, map[string]any{"playlist_id": pid, "snapshot_id": sid, "position": pos})
	}
	rows.Close()

	// Saved.
	rows, err = db.Query(`SELECT user_id, saved_at FROM saved_tracks WHERE track_id = ?`, trackID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var userID, savedAt string
		if err := rows.Scan(&userID, &savedAt); err != nil {
			rows.Close()
			return nil, err
		}
		result.SavedTracks = append(result.SavedTracks, map[string]any{"user_id": userID, "saved_at": savedAt})
	}
	rows.Close()

	// Play history.
	rows, err = db.Query(`SELECT played_at, COALESCE(context_uri, ''), COALESCE(context_type, '')
		FROM play_history WHERE track_id = ? ORDER BY played_at DESC`, trackID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var playedAt, contextURI, contextType string
		if err := rows.Scan(&playedAt, &contextURI, &contextType); err != nil {
			rows.Close()
			return nil, err
		}
		result.PlayHistory = append(result.PlayHistory, map[string]any{"played_at": playedAt, "context_uri": contextURI, "context_type": contextType})
	}
	rows.Close()

	// Top snapshots.
	rows, err = db.Query(`SELECT captured_at, time_range, position FROM top_tracks_snapshot WHERE track_id = ? ORDER BY captured_at DESC`, trackID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var capturedAt, timeRange string
		var pos int
		if err := rows.Scan(&capturedAt, &timeRange, &pos); err != nil {
			rows.Close()
			return nil, err
		}
		result.TopSnapshots = append(result.TopSnapshots, map[string]any{"captured_at": capturedAt, "time_range": timeRange, "position": pos})
	}
	rows.Close()

	result.TotalHits = len(result.Playlists) + len(result.SavedTracks) + len(result.PlayHistory) + len(result.TopSnapshots)
	return result, nil
}

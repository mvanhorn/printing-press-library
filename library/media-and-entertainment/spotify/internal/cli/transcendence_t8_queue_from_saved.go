// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// T8 — Agent-friendly queue-from-saved.
//
//	queue from-saved [--limit N]
//
// Selects the N most recently saved tracks from local saved_tracks and
// POSTs each URI to /me/player/queue. Surfaces 403 PREMIUM_REQUIRED
// cleanly. Per-artist and per-playlist filters are not yet implemented;
// the saved_tracks schema is (user_id, track_id, saved_at) and does not
// carry artist or playlist linkage. Adding filters requires a join
// against a tracks-cache table populated by sync.
//
// PATCH (fix-queue-from-saved-remove-unimplemented-flags): the --artist
// and --playlist flags were removed in this print; they were registered
// but never referenced in the SQL query and misled callers.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil"
)

func newQueueCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Local queue helpers (queue from-saved, etc.)",
	}
	cmd.AddCommand(newQueueFromSavedCmd(flags))
	return cmd
}

func newQueueFromSavedCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "from-saved [--limit N]",
		Short:   "Queue N tracks from your saved library (Premium only — playback writes need Premium)",
		Example: "  spotify-pp-cli queue from-saved --limit 10",
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				limit = 10
			}

			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.DB().Query(`SELECT track_id FROM saved_tracks ORDER BY saved_at DESC LIMIT ?`, limit)
			if err != nil {
				return fmt.Errorf("reading saved_tracks: %w", err)
			}
			var trackIDs []string
			for rows.Next() {
				var tid string
				if err := rows.Scan(&tid); err != nil {
					rows.Close()
					return err
				}
				trackIDs = append(trackIDs, tid)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}

			plan := make([]string, 0, len(trackIDs))
			for _, tid := range trackIDs {
				plan = append(plan, "spotify:track:"+tid)
			}

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":     true,
					"would_queue": plan,
					"would_count": len(plan),
				}, flags)
			}
			if len(plan) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"queued": 0,
					"hint":   "no saved tracks in local store — run 'spotify-pp-cli sync' first",
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			queued := 0
			for _, uri := range plan {
				_, _, err := c.Post("/me/player/queue?uri="+uri, nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				queued++
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"queued": queued,
				"uris":   plan,
			}, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Max tracks to queue")
	return cmd
}

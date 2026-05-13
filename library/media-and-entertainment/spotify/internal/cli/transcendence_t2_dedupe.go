// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// T2 — ISRC-aware playlist dedupe.
//
//	playlists dedupe <playlist-id> [--by isrc|track-id|title-artist] [--apply]
//
// Reports without applying by default. With --apply, calls the Spotify
// remove-tracks endpoint snapshot-guarded. ISRC mode catches the
// album/single/EP/deluxe-reissue dupe class that ID-based dedupe misses.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil"
)

func newPlaylistsDedupeCmd(flags *rootFlags) *cobra.Command {
	var byMode string
	var apply bool
	cmd := &cobra.Command{
		Use:   "dedupe <playlist-id> [--by isrc|track-id|title-artist] [--apply]",
		Short: "Find duplicate tracks in a playlist (by ISRC by default)",
		Long: `Detects duplicate tracks in a playlist. ISRC mode (the default) catches the
album/single/EP/deluxe-reissue dupe class that bare-ID dedupe misses.

Without --apply, only reports the dupe sets — no API mutation is made.
With --apply, calls Spotify's remove-tracks endpoint with a snapshot guard.`,
		Example: "  spotify-pp-cli playlists dedupe 37i9dQZF1DXcBWIGoYBM5M --by isrc",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			playlistID := bareID(args[0])
			switch byMode {
			case "isrc", "track-id", "title-artist":
			default:
				return usageErr(fmt.Errorf("--by must be one of: isrc, track-id, title-artist"))
			}

			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":     true,
					"playlist_id": playlistID,
					"by":          byMode,
					"would_apply": apply,
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/playlists/"+playlistID, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var pl struct {
				ID         string `json:"id"`
				SnapshotID string `json:"snapshot_id"`
				Tracks     struct {
					Items []struct {
						AddedAt string `json:"added_at"`
						Track   struct {
							ID      string `json:"id"`
							URI     string `json:"uri"`
							Name    string `json:"name"`
							Artists []struct {
								Name string `json:"name"`
							} `json:"artists"`
							ExternalIDs struct {
								ISRC string `json:"isrc"`
							} `json:"external_ids"`
						} `json:"track"`
					} `json:"items"`
				} `json:"tracks"`
			}
			if err := json.Unmarshal(data, &pl); err != nil {
				return fmt.Errorf("decoding playlist: %w", err)
			}
			now := time.Now().UTC()
			for i, item := range pl.Tracks.Items {
				addedAt, _ := time.Parse(time.RFC3339, item.AddedAt)
				_ = db.InsertPlaylistSnapshotTrack(
					pl.ID, pl.SnapshotID, now, i,
					item.Track.ID, item.Track.URI, item.Track.Name, item.Track.ExternalIDs.ISRC,
					addedAt, "",
				)
			}

			// Build duplicate sets from the in-memory data so we don't
			// depend on whether the local snapshot table has prior rows.
			type trackRow struct {
				Pos  int    `json:"position"`
				ID   string `json:"track_id"`
				URI  string `json:"uri"`
				Name string `json:"name"`
			}
			groups := map[string][]trackRow{}
			for i, item := range pl.Tracks.Items {
				var key string
				switch byMode {
				case "isrc":
					key = strings.ToUpper(item.Track.ExternalIDs.ISRC)
				case "track-id":
					key = item.Track.ID
				case "title-artist":
					parts := []string{strings.ToLower(item.Track.Name)}
					for _, a := range item.Track.Artists {
						parts = append(parts, strings.ToLower(a.Name))
					}
					key = strings.Join(parts, "|")
				}
				if key == "" {
					continue
				}
				groups[key] = append(groups[key], trackRow{Pos: i, ID: item.Track.ID, URI: item.Track.URI, Name: item.Track.Name})
			}
			dupes := []map[string]any{}
			toRemove := []map[string]any{}
			for key, rows := range groups {
				if len(rows) < 2 {
					continue
				}
				dupes = append(dupes, map[string]any{
					"key":         key,
					"occurrences": rows,
				})
				// Drop all but the first occurrence.
				for _, r := range rows[1:] {
					toRemove = append(toRemove, map[string]any{"uri": r.URI, "positions": []int{r.Pos}})
				}
			}

			out := map[string]any{
				"playlist_id":  playlistID,
				"snapshot_id":  pl.SnapshotID,
				"by":           byMode,
				"dupe_sets":    len(dupes),
				"removed":      0,
				"would_remove": len(toRemove),
				"groups":       dupes,
			}

			if !apply {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			// Apply: snapshot-aware DELETE /playlists/{id}/tracks. The
			// tracks + snapshot_id payload is required by Spotify and goes
			// in the JSON body (not the URL). The original c.Delete call
			// silently dropped the body, leaving every --apply as a no-op
			// against the API.
			body := map[string]any{
				"tracks":      toRemove,
				"snapshot_id": pl.SnapshotID,
			}
			_, _, err = c.DeleteWithBody("/playlists/"+playlistID+"/tracks", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out["removed"] = len(toRemove)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&byMode, "by", "isrc", "Match mode: isrc, track-id, or title-artist")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually remove duplicates (otherwise just reports)")
	return cmd
}

// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Hand-built `sync-extras` command. Populates the transcendence-feature
// tables (saved_tracks, saved_albums, followed_artists, top_*_snapshot,
// play_history, devices_seen) from the live Spotify API. The generator's
// `sync` command handles the spec-derived resource tables; this command
// handles the per-user denormalized state those tables don't capture.
//
// Run after `spotify-pp-cli sync` (which populates raw API resources) or
// independently — the two are non-overlapping.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil"
)

func newSyncExtrasCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sync-extras",
		Short:   "Sync the transcendence-feature tables (saved, followed, top, history, devices)",
		Example: "  spotify-pp-cli sync-extras",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "would_sync": []string{
					"saved_tracks", "saved_albums", "followed_artists",
					"top_tracks_snapshot", "top_artists_snapshot",
					"play_history", "devices_seen",
				}}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			userID, err := fetchCurrentUserID(c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			counts := map[string]int{}
			now := time.Now().UTC()

			// Saved tracks: paginated /me/tracks.
			if items, err := fetchAllPaged(c, "/me/tracks", map[string]string{"limit": "50"}, 0); err == nil {
				for _, item := range items {
					var row struct {
						AddedAt string `json:"added_at"`
						Track   struct {
							ID string `json:"id"`
						} `json:"track"`
					}
					if json.Unmarshal(item, &row) == nil && row.Track.ID != "" {
						added, _ := time.Parse(time.RFC3339, row.AddedAt)
						if added.IsZero() {
							added = now
						}
						_ = db.InsertSavedTrack(userID, row.Track.ID, added)
						counts["saved_tracks"]++
					}
				}
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: saved tracks: %v\n", err)
			}

			// Saved albums.
			if items, err := fetchAllPaged(c, "/me/albums", map[string]string{"limit": "50"}, 0); err == nil {
				for _, item := range items {
					var row struct {
						AddedAt string `json:"added_at"`
						Album   struct {
							ID string `json:"id"`
						} `json:"album"`
					}
					if json.Unmarshal(item, &row) == nil && row.Album.ID != "" {
						added, _ := time.Parse(time.RFC3339, row.AddedAt)
						if added.IsZero() {
							added = now
						}
						_ = db.InsertSavedAlbum(userID, row.Album.ID, added)
						counts["saved_albums"]++
					}
				}
			}

			// Followed artists. The endpoint shape is /me/following?type=artist
			// and returns { artists: { items: [], next, cursors }}.
			{
				params := map[string]string{"type": "artist", "limit": "50"}
				cursor := ""
				for {
					if cursor != "" {
						params["after"] = cursor
					}
					data, err := c.Get("/me/following", params)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: followed artists: %v\n", err)
						break
					}
					var resp struct {
						Artists struct {
							Items []struct {
								ID   string `json:"id"`
								Name string `json:"name"`
							} `json:"items"`
							Cursors struct {
								After string `json:"after"`
							} `json:"cursors"`
						} `json:"artists"`
					}
					if json.Unmarshal(data, &resp) != nil {
						break
					}
					for _, a := range resp.Artists.Items {
						_ = db.InsertFollowedArtist(userID, a.ID, a.Name, now)
						counts["followed_artists"]++
					}
					if resp.Artists.Cursors.After == "" || len(resp.Artists.Items) == 0 {
						break
					}
					cursor = resp.Artists.Cursors.After
				}
			}

			// Top tracks (3 time ranges, snapshot each).
			for _, tr := range []string{"short_term", "medium_term", "long_term"} {
				data, err := c.Get("/me/top/tracks", map[string]string{"time_range": tr, "limit": "50"})
				if err != nil {
					continue
				}
				var resp struct {
					Items []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"items"`
				}
				if json.Unmarshal(data, &resp) == nil {
					for i, t := range resp.Items {
						_ = db.InsertTopTrack(now, tr, i, t.ID, t.Name)
						counts["top_tracks_snapshot"]++
					}
				}
			}

			// Top artists.
			for _, tr := range []string{"short_term", "medium_term", "long_term"} {
				data, err := c.Get("/me/top/artists", map[string]string{"time_range": tr, "limit": "50"})
				if err != nil {
					continue
				}
				var resp struct {
					Items []struct {
						ID     string   `json:"id"`
						Name   string   `json:"name"`
						Genres []string `json:"genres"`
					} `json:"items"`
				}
				if json.Unmarshal(data, &resp) == nil {
					for i, a := range resp.Items {
						_ = db.InsertTopArtist(now, tr, i, a.ID, a.Name, a.Genres)
						counts["top_artists_snapshot"]++
					}
				}
			}

			// Recently played.
			{
				data, err := c.Get("/me/player/recently-played", map[string]string{"limit": "50"})
				if err == nil {
					var resp struct {
						Items []struct {
							PlayedAt string `json:"played_at"`
							Track    struct {
								ID   string `json:"id"`
								Name string `json:"name"`
							} `json:"track"`
							Context struct {
								URI  string `json:"uri"`
								Type string `json:"type"`
							} `json:"context"`
						} `json:"items"`
					}
					if json.Unmarshal(data, &resp) == nil {
						for _, item := range resp.Items {
							played, _ := time.Parse(time.RFC3339, item.PlayedAt)
							if played.IsZero() {
								continue
							}
							_ = db.InsertPlayHistory(played, item.Track.ID, item.Track.Name, item.Context.URI, item.Context.Type)
							counts["play_history"]++
						}
					}
				}
			}

			// Devices.
			{
				data, err := c.Get("/me/player/devices", nil)
				if err == nil {
					var resp struct {
						Devices []struct {
							ID            string `json:"id"`
							Name          string `json:"name"`
							Type          string `json:"type"`
							IsActive      bool   `json:"is_active"`
							VolumePercent int    `json:"volume_percent"`
						} `json:"devices"`
					}
					if json.Unmarshal(data, &resp) == nil {
						for _, d := range resp.Devices {
							if d.ID == "" {
								continue
							}
							_ = db.InsertDeviceSeen(d.ID, d.Name, d.Type, d.IsActive, d.VolumePercent, now)
							counts["devices_seen"]++
						}
					}
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"user_id": userID,
				"counts":  counts,
			}, flags)
		},
	}
	return cmd
}

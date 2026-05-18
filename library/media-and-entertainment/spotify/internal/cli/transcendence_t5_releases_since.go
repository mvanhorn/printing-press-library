// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// T5 — Release-Radar replacement.
//
//	releases since <date> [--from followed]
//
// Iterates `followed_artists`, calls /artists/{id}/albums per artist,
// filters to releases on/after the date. Replaces Spotify's algorithmic
// Release Radar (effectively unavailable to new apps) with a deterministic
// feed the user controls.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil"
)

func newReleasesSinceCmd(flags *rootFlags) *cobra.Command {
	var fromArg string
	var includeGroups string
	var limit int
	cmd := &cobra.Command{
		Use:   "since <date>",
		Short: "List new releases from followed artists since a date",
		Long: `Iterates followed_artists in the local store, fetches each artist's albums
filtered by min_release_date, and returns the merged set sorted by release
date descending. Replaces Spotify's algorithmic Release Radar with a
deterministic feed.

Requires followed_artists to be populated (call 'sync' first or run
'followed-artists list' which caches into the same table).`,
		Example:     "  spotify-pp-cli releases since 2026-04-01",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			since, err := time.Parse("2006-01-02", args[0])
			if err != nil {
				return usageErr(fmt.Errorf("date must be YYYY-MM-DD: %w", err))
			}
			if fromArg != "followed" {
				return usageErr(fmt.Errorf("--from must be 'followed' (only source supported)"))
			}

			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "since": args[0]}, flags)
			}

			rows, err := db.DB().Query(`SELECT artist_id, COALESCE(artist_name, '') FROM followed_artists`)
			if err != nil {
				return fmt.Errorf("reading followed_artists: %w", err)
			}
			type artistRef struct {
				ID, Name string
			}
			var artists []artistRef
			for rows.Next() {
				var a artistRef
				if err := rows.Scan(&a.ID, &a.Name); err != nil {
					rows.Close()
					return err
				}
				artists = append(artists, a)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			if len(artists) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"since":    args[0],
					"releases": []any{},
					"hint":     "no followed artists in local store — run 'spotify-pp-cli sync' to populate",
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type release struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				ReleaseDate string `json:"release_date"`
				ArtistID    string `json:"artist_id"`
				ArtistName  string `json:"artist_name"`
				AlbumType   string `json:"album_type"`
				URI         string `json:"uri"`
			}
			// PATCH (fix-releases-since-pagination):
			// Paginate /artists/{id}/albums per artist instead of taking the
			// first page only. Spotify returns albums newest-first, so as soon
			// as a page's last item is older than `since`, every subsequent
			// page is older too and we can stop. Without this, any artist with
			// more than 10 releases in the window silently loses the tail.
			// limit=10 dodges Spotify's post-2024-11-27 new-app cap on this
			// endpoint (the documented max=50 returns HTTP 400 "Invalid limit").
			var releases []release
			for _, a := range artists {
				cursor := "/artists/" + a.ID + "/albums"
				cursorParams := map[string]string{
					"include_groups": includeGroups,
					"limit":          "10",
				}
			pageLoop:
				for {
					data, err := c.Get(cursor, cursorParams)
					if err != nil {
						// Skip artists that 404 etc.; report inline.
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: artist %s skipped: %v\n", a.ID, err)
						break pageLoop
					}
					var resp struct {
						Items []struct {
							ID          string `json:"id"`
							Name        string `json:"name"`
							ReleaseDate string `json:"release_date"`
							AlbumType   string `json:"album_type"`
							URI         string `json:"uri"`
						} `json:"items"`
						Next string `json:"next"`
					}
					if err := json.Unmarshal(data, &resp); err != nil {
						break pageLoop
					}
					stopEarly := false
					for _, alb := range resp.Items {
						rd, err := parseSpotifyReleaseDate(alb.ReleaseDate)
						if err != nil {
							continue
						}
						if rd.Before(since) {
							// Newest-first ordering means everything after this
							// page is older too; finish the current page (it
							// may contain mixed dates) then stop.
							stopEarly = true
							continue
						}
						releases = append(releases, release{
							ID:          alb.ID,
							Name:        alb.Name,
							ReleaseDate: alb.ReleaseDate,
							ArtistID:    a.ID,
							ArtistName:  a.Name,
							AlbumType:   alb.AlbumType,
							URI:         alb.URI,
						})
					}
					if stopEarly || resp.Next == "" {
						break pageLoop
					}
					nextPath, nextParams, err := splitURL(resp.Next)
					if err != nil {
						break pageLoop
					}
					cursor = nextPath
					cursorParams = nextParams
				}
			}

			sort.SliceStable(releases, func(i, j int) bool {
				return releases[i].ReleaseDate > releases[j].ReleaseDate
			})
			if limit > 0 && len(releases) > limit {
				releases = releases[:limit]
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"since":    args[0],
				"artists":  len(artists),
				"count":    len(releases),
				"releases": releases,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&fromArg, "from", "followed", "Source: followed (the only supported value today)")
	cmd.Flags().StringVar(&includeGroups, "include-groups", "album,single", "Comma-separated Spotify album groups to include")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap on returned releases (0 for unlimited)")
	return cmd
}

// parseSpotifyReleaseDate accepts the three precision levels Spotify
// returns: year-only, year-month, full date.
func parseSpotifyReleaseDate(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", "2006-01", "2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable release_date: %s", s)
}

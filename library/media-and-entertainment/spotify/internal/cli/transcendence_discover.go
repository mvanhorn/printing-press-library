// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Transcendence commands T9-T12, all under the `discover` subtree:
//
//   discover artists           — genre-walked artist discovery (T9)
//   discover via-playlists     — co-occurrence via public playlists (T10)
//   discover artist-gaps       — artist deep-dive vs local saved library (T11)
//   discover new-releases      — filter global new releases by your genres (T12)
//
// All four lean on data the deprecated /recommendations and /related-artists
// endpoints used to provide, but reconstruct that data from endpoints that
// still work for new apps (search, /browse/new-releases, /artists/{id}/albums).

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil"
)

func newDiscoverCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Novel discovery commands (artists, via-playlists, artist-gaps, new-releases)",
	}
	cmd.AddCommand(newDiscoverArtistsCmd(flags))
	cmd.AddCommand(newDiscoverViaPlaylistsCmd(flags))
	cmd.AddCommand(newDiscoverArtistGapsCmd(flags))
	cmd.AddCommand(newDiscoverNewReleasesCmd(flags))
	return cmd
}

// ---------- T9: discover artists ----------------------------------------------

func newDiscoverArtistsCmd(flags *rootFlags) *cobra.Command {
	var seed string
	var limit int
	var excludeFollowed bool
	cmd := &cobra.Command{
		Use:   "artists [--seed top|saved|followed] [--limit N] [--exclude-followed]",
		Short: "Find artists matching the genres of your top/saved/followed artists",
		Long: `Walks the genres of your top/saved/followed artists, searches Spotify for
each unique genre, ranks unfollowed artists by popularity. Replaces the
deprecated /recommendations endpoint for the "artists I might like" question.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  spotify-pp-cli discover artists --seed top --limit 20",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch seed {
			case "top", "saved", "followed":
			default:
				return usageErr(fmt.Errorf("--seed must be top, saved, or followed"))
			}

			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "seed": seed}, flags)
			}

			// Collect genres from the seed table.
			genres, err := readSeedGenres(db.DB(), seed)
			if err != nil {
				return err
			}
			if len(genres) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"seed":       seed,
					"candidates": []any{},
					"hint":       "no genres in seed source — run 'spotify-pp-cli sync' or 'top artists' first",
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			followed, err := readFollowedSet(db.DB())
			if err != nil {
				return err
			}

			candidates, err := searchArtistsByGenres(c, genres, followed, excludeFollowed)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if limit > 0 && len(candidates) > limit {
				candidates = candidates[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"seed":       seed,
				"genres":     genres,
				"candidates": candidates,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&seed, "seed", "top", "Source genres: top, saved, or followed")
	cmd.Flags().IntVar(&limit, "limit", 20, "Cap on returned candidates")
	cmd.Flags().BoolVar(&excludeFollowed, "exclude-followed", true, "Exclude artists already followed")
	return cmd
}

func readSeedGenres(db storeQueryer, seed string) ([]string, error) {
	switch seed {
	case "top":
		rows, err := db.Query(`SELECT artist_genres FROM top_artists_snapshot
			WHERE captured_at = (SELECT MAX(captured_at) FROM top_artists_snapshot)`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		seen := map[string]bool{}
		var out []string
		for rows.Next() {
			var raw string
			if err := rows.Scan(&raw); err != nil {
				return nil, err
			}
			var genres []string
			_ = json.Unmarshal([]byte(raw), &genres)
			for _, g := range genres {
				if g != "" && !seen[g] {
					seen[g] = true
					out = append(out, g)
				}
			}
		}
		return out, rows.Err()
	}
	// 'saved' and 'followed' sources currently fall back to top_artists genres
	// since saved_albums/followed_artists don't store genre lists yet. Future
	// improvement: read raw artist JSON from the 'artists' table.
	return readSeedGenres(db, "top")
}

func readFollowedSet(db storeQueryer) (map[string]bool, error) {
	rows, err := db.Query(`SELECT artist_id FROM followed_artists`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func searchArtistsByGenres(c *client.Client, genres []string, followed map[string]bool, excludeFollowed bool) ([]map[string]any, error) {
	type candidate struct {
		ID         string
		Name       string
		Popularity int
		Genres     []string
		MatchGenre string
	}
	collected := map[string]candidate{}
	for _, g := range genres {
		data, err := c.Get("/search", map[string]string{
			"q":     fmt.Sprintf("genre:\"%s\"", g),
			"type":  "artist",
			"limit": "20",
		})
		if err != nil {
			return nil, err
		}
		var resp struct {
			Artists struct {
				Items []struct {
					ID         string   `json:"id"`
					Name       string   `json:"name"`
					Popularity int      `json:"popularity"`
					Genres     []string `json:"genres"`
				} `json:"items"`
			} `json:"artists"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}
		for _, a := range resp.Artists.Items {
			if excludeFollowed && followed[a.ID] {
				continue
			}
			if existing, ok := collected[a.ID]; ok {
				if a.Popularity > existing.Popularity {
					existing.Popularity = a.Popularity
				}
				collected[a.ID] = existing
				continue
			}
			collected[a.ID] = candidate{
				ID:         a.ID,
				Name:       a.Name,
				Popularity: a.Popularity,
				Genres:     a.Genres,
				MatchGenre: g,
			}
		}
	}
	out := make([]map[string]any, 0, len(collected))
	for _, c := range collected {
		out = append(out, map[string]any{
			"id":          c.ID,
			"name":        c.Name,
			"popularity":  c.Popularity,
			"genres":      c.Genres,
			"match_genre": c.MatchGenre,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i]["popularity"].(int) > out[j]["popularity"].(int) })
	return out, nil
}

// ---------- T10: discover via-playlists --------------------------------------

func newDiscoverViaPlaylistsCmd(flags *rootFlags) *cobra.Command {
	var minCo int
	var limit int
	cmd := &cobra.Command{
		Use:   "via-playlists <seed-artist-id> [--min-cooccurrence N] [--limit N]",
		Short: "Find artists frequently co-curated with a seed artist (public playlist co-occurrence)",
		Long: `Searches public playlists matching the seed artist's name, fetches each
playlist's tracks, counts co-occurring artists, and ranks by frequency.
Spotify's /related-artists endpoint is deprecated for new apps — public
playlists become the graph substrate instead.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  spotify-pp-cli discover via-playlists 4NHQUGzhtTLFvgF5SZesLK --min-cooccurrence 3",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			seedID := bareID(args[0])

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "seed_artist_id": seedID}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Step 1: get seed artist's name.
			artistData, err := c.Get("/artists/"+seedID, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var artist struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			_ = json.Unmarshal(artistData, &artist)

			// PATCH (fix-discover-spotify-new-app-limit-caps):
			// Step 2: search public playlists by name. Spotify's
			// post-2024-11-27 new-app constraint caps limit at ~10 on
			// /search (the documented max of 50 returns "Invalid limit"
			// 400), so we keep this small.
			searchData, err := c.Get("/search", map[string]string{
				"q":     artist.Name,
				"type":  "playlist",
				"limit": "10",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var srch struct {
				Playlists struct {
					Items []struct {
						ID string `json:"id"`
					} `json:"items"`
				} `json:"playlists"`
			}
			_ = json.Unmarshal(searchData, &srch)

			// Step 3: fetch tracks for each playlist, count artists.
			counts := map[string]int{}
			names := map[string]string{}
			for _, pl := range srch.Playlists.Items {
				items, err := c.Get("/playlists/"+pl.ID+"/tracks", map[string]string{"limit": "10"})
				if err != nil {
					continue
				}
				var p struct {
					Items []struct {
						Track struct {
							Artists []struct {
								ID   string `json:"id"`
								Name string `json:"name"`
							} `json:"artists"`
						} `json:"track"`
					} `json:"items"`
				}
				_ = json.Unmarshal(items, &p)
				seenInPlaylist := map[string]bool{}
				for _, item := range p.Items {
					for _, a := range item.Track.Artists {
						if a.ID == "" || a.ID == seedID || seenInPlaylist[a.ID] {
							continue
						}
						seenInPlaylist[a.ID] = true
						counts[a.ID]++
						names[a.ID] = a.Name
					}
				}
			}

			type ranked struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Count int    `json:"cooccurrence_count"`
			}
			var out []ranked
			for id, c := range counts {
				if c < minCo {
					continue
				}
				out = append(out, ranked{ID: id, Name: names[id], Count: c})
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"seed_artist":       map[string]any{"id": seedID, "name": artist.Name},
				"playlists_scanned": len(srch.Playlists.Items),
				"cooccurring":       out,
			}, flags)
		},
	}
	cmd.Flags().IntVar(&minCo, "min-cooccurrence", 2, "Minimum playlists an artist must co-occur in to surface")
	cmd.Flags().IntVar(&limit, "limit", 30, "Cap on returned candidates")
	return cmd
}

// ---------- T11: discover artist-gaps ----------------------------------------

func newDiscoverArtistGapsCmd(flags *rootFlags) *cobra.Command {
	var show string
	var includeGroups string
	cmd := &cobra.Command{
		Use:   "artist-gaps <artist-id> [--show all|saved|unsaved] [--include-groups <list>]",
		Short: "Show an artist's full discography against your saved library coverage",
		Long: `Lists all albums by an artist with each marked as saved/unsaved in your
local library. Answers "I love this artist — what have I missed?"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  spotify-pp-cli discover artist-gaps 4NHQUGzhtTLFvgF5SZesLK --show unsaved",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			artistID := bareID(args[0])
			switch show {
			case "all", "saved", "unsaved":
			default:
				return usageErr(fmt.Errorf("--show must be all, saved, or unsaved"))
			}

			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "artist_id": artistID}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Spotify's new-app constraint caps /artists/{id}/albums at
			// limit ~10 (the documented max of 50 returns "Invalid limit"
			// 400). Page via fetchAllPaged, which follows the response's
			// `next` URL — cap at 100 albums per artist.
			rawItems, err := fetchAllPaged(c, "/artists/"+artistID+"/albums", map[string]string{
				"include_groups": includeGroups,
				"limit":          "10",
			}, 100)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				Items []struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					ReleaseDate string `json:"release_date"`
					AlbumType   string `json:"album_type"`
				} `json:"items"`
			}
			for _, raw := range rawItems {
				var item struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					ReleaseDate string `json:"release_date"`
					AlbumType   string `json:"album_type"`
				}
				if json.Unmarshal(raw, &item) == nil {
					resp.Items = append(resp.Items, item)
				}
			}

			savedSet, err := readSavedAlbumsSet(db.DB())
			if err != nil {
				return err
			}

			type albumRow struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				ReleaseDate string `json:"release_date"`
				AlbumType   string `json:"album_type"`
				Saved       bool   `json:"saved"`
			}
			var out []albumRow
			for _, alb := range resp.Items {
				saved := savedSet[alb.ID]
				switch show {
				case "saved":
					if !saved {
						continue
					}
				case "unsaved":
					if saved {
						continue
					}
				}
				out = append(out, albumRow{
					ID:          alb.ID,
					Name:        alb.Name,
					ReleaseDate: alb.ReleaseDate,
					AlbumType:   alb.AlbumType,
					Saved:       saved,
				})
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].ReleaseDate < out[j].ReleaseDate })

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"artist_id":   artistID,
				"total":       len(resp.Items),
				"shown":       len(out),
				"discography": out,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&show, "show", "all", "Filter: all, saved, or unsaved")
	cmd.Flags().StringVar(&includeGroups, "include-groups", "album,single,compilation", "Spotify album groups to include")
	return cmd
}

func readSavedAlbumsSet(db storeQueryer) (map[string]bool, error) {
	rows, err := db.Query(`SELECT album_id FROM saved_albums`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// ---------- T12: discover new-releases ---------------------------------------

func newDiscoverNewReleasesCmd(flags *rootFlags) *cobra.Command {
	var seedFrom string
	var days int
	var excludeFollowed bool
	var limit int
	cmd := &cobra.Command{
		Use:   "new-releases [--seed-from top|followed] [--days N] [--exclude-followed] [--limit N]",
		Short: "Filter Spotify's global new-releases feed to artists sharing your genres",
		Long: `Pulls /browse/new-releases, looks up each release's artists' genres, and
keeps only releases whose artists share at least one genre with your top
(or followed) artists.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  spotify-pp-cli discover new-releases --seed-from top --days 14",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch seedFrom {
			case "top", "followed":
			default:
				return usageErr(fmt.Errorf("--seed-from must be top or followed"))
			}

			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "seed_from": seedFrom}, flags)
			}

			seedGenres, err := readSeedGenres(db.DB(), seedFrom)
			if err != nil {
				return err
			}
			seedGenreSet := map[string]bool{}
			for _, g := range seedGenres {
				seedGenreSet[strings.ToLower(g)] = true
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/browse/new-releases", map[string]string{"limit": "50"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				Albums struct {
					Items []struct {
						ID          string `json:"id"`
						Name        string `json:"name"`
						ReleaseDate string `json:"release_date"`
						Artists     []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"artists"`
					} `json:"items"`
				} `json:"albums"`
			}
			_ = json.Unmarshal(data, &resp)

			followed, _ := readFollowedSet(db.DB())
			type relRow struct {
				ID            string   `json:"id"`
				Name          string   `json:"name"`
				ReleaseDate   string   `json:"release_date"`
				Artists       []string `json:"artists"`
				MatchedGenres []string `json:"matched_genres"`
			}
			var out []relRow
			for _, alb := range resp.Albums.Items {
				if days > 0 {
					if rd, err := parseSpotifyReleaseDate(alb.ReleaseDate); err == nil {
						if rd.Unix() < time.Now().AddDate(0, 0, -days).Unix() {
							continue
						}
					}
				}
				skipDueToFollowed := false
				if excludeFollowed {
					for _, a := range alb.Artists {
						if followed[a.ID] {
							skipDueToFollowed = true
							break
						}
					}
				}
				if skipDueToFollowed {
					continue
				}
				// Look up artist genres via /artists batched.
				artistIDs := make([]string, 0, len(alb.Artists))
				artistNames := make([]string, 0, len(alb.Artists))
				for _, a := range alb.Artists {
					artistIDs = append(artistIDs, a.ID)
					artistNames = append(artistNames, a.Name)
				}
				if len(artistIDs) == 0 {
					continue
				}
				matchedGenres := artistGenreMatches(c, artistIDs, seedGenreSet)
				if len(seedGenreSet) > 0 && len(matchedGenres) == 0 {
					continue
				}
				out = append(out, relRow{
					ID:            alb.ID,
					Name:          alb.Name,
					ReleaseDate:   alb.ReleaseDate,
					Artists:       artistNames,
					MatchedGenres: matchedGenres,
				})
			}
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"seed_from":      seedFrom,
				"seed_genres":    seedGenres,
				"days":           days,
				"matching_count": len(out),
				"releases":       out,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&seedFrom, "seed-from", "top", "Source genres: top or followed")
	cmd.Flags().IntVar(&days, "days", 30, "Only releases from the last N days (0 for all)")
	cmd.Flags().BoolVar(&excludeFollowed, "exclude-followed", false, "Exclude releases from artists you already follow")
	cmd.Flags().IntVar(&limit, "limit", 30, "Cap on returned releases")
	return cmd
}

func artistGenreMatches(c *client.Client, artistIDs []string, seedGenres map[string]bool) []string {
	if len(seedGenres) == 0 {
		return nil
	}
	data, err := c.Get("/artists", map[string]string{"ids": strings.Join(artistIDs, ",")})
	if err != nil {
		return nil
	}
	var resp struct {
		Artists []struct {
			Genres []string `json:"genres"`
		} `json:"artists"`
	}
	_ = json.Unmarshal(data, &resp)
	seen := map[string]bool{}
	var matched []string
	for _, a := range resp.Artists {
		for _, g := range a.Genres {
			lg := strings.ToLower(g)
			if seedGenres[lg] && !seen[lg] {
				seen[lg] = true
				matched = append(matched, g)
			}
		}
	}
	return matched
}

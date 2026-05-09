// Copyright 2026 twidtwid. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(catalog/peloton): novel feature — discoveries.
// Walks the most-recent N workouts and collects every in-class song the user
// liked, deduped by song id with a times_played counter. No Peloton UI
// surfaces this view; likes are stored Peloton-side per ride playback rather
// than per-song globally. Recorded in .printing-press-patches.json.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/client"
)

// discovery is one in-class song the user liked, paired with the workout +
// ride context that surfaced it.
type discovery struct {
	SongID      string   `json:"song_id"`
	Title       string   `json:"title"`
	Artists     []string `json:"artists"`
	Album       string   `json:"album"`
	WorkoutID   string   `json:"workout_id"`
	WorkoutDate string   `json:"workout_date"`
	RideID      string   `json:"ride_id"`
	RideTitle   string   `json:"ride_title"`
	TimesPlayed int      `json:"times_played"`
}

// rideFetchConcurrency caps the parallel /api/ride/{id}/details calls.
// Peloton hasn't published a rate limit; 4 in flight is what the SPA uses
// when prefetching adjacent ride pages.
const rideFetchConcurrency = 4

func newDiscoveriesCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "discoveries",
		Short: "List in-class songs you liked across recent workouts",
		Long: `Scans your most-recent --limit workouts (default 30), fetches each
ride's playlist, and emits the songs flagged liked=true in-class. Songs liked
in multiple rides are deduped by song id; times_played counts how many of the
scanned rides played the song.`,
		Example: `  peloton-pp-cli discoveries
  peloton-pp-cli discoveries --limit 100 --agent
  peloton-pp-cli discoveries --json | jq '.[] | "\(.title) — \(.artists | join(", "))"'`,
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return apiErr(err)
			}

			userID, err := resolveCurrentUserID(c)
			if err != nil {
				return apiErr(fmt.Errorf("resolving user id: %w", err))
			}

			workouts, err := fetchRecentWorkouts(c, userID, limit)
			if err != nil {
				return apiErr(fmt.Errorf("fetching workouts: %w", err))
			}

			ds := collectDiscoveries(c, workouts)
			return printJSONFiltered(cmd.OutOrStdout(), ds, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 30, "How many recent workouts to scan")
	return cmd
}

// resolveCurrentUserID hits /api/me and returns the bare user id (no
// auth0|<id> prefix). Used by discoveries so the user doesn't have to
// pass their userID explicitly.
func resolveCurrentUserID(c *client.Client) (string, error) {
	raw, err := c.Get("/me", nil)
	if err != nil {
		return "", fmt.Errorf("/me: %w", err)
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &me); err != nil {
		return "", fmt.Errorf("decoding /me: %w", err)
	}
	if me.ID == "" {
		return "", fmt.Errorf("/me returned empty id")
	}
	return me.ID, nil
}

// listedWorkout is the tiny projection of /api/user/{id}/workouts response
// data we need for the discoveries walk: the workout id (for downstream
// context), the workout date (for display), and the ride id (the actual
// thing we'll fetch playlist for).
type listedWorkout struct {
	ID          string
	WorkoutDate string
	RideID      string
}

func fetchRecentWorkouts(c *client.Client, userID string, limit int) ([]listedWorkout, error) {
	if limit <= 0 {
		limit = 30
	}
	const pageSize = 20

	type rawWorkout struct {
		ID        string `json:"id"`
		StartTime int64  `json:"start_time"`
		Ride      struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"ride"`
	}

	var out []listedWorkout
	for page := 0; len(out) < limit; page++ {
		params := map[string]string{
			"page":  strconv.Itoa(page),
			"limit": strconv.Itoa(pageSize),
			"joins": "ride,ride.instructor",
		}
		raw, err := c.Get("/user/"+userID+"/workouts", params)
		if err != nil {
			return out, err
		}
		var resp struct {
			Data []rawWorkout `json:"data"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return out, fmt.Errorf("decoding page %d: %w", page, err)
		}
		if len(resp.Data) == 0 {
			break
		}
		for _, w := range resp.Data {
			date := ""
			if w.StartTime > 0 {
				date = formatDate(w.StartTime)
			}
			out = append(out, listedWorkout{ID: w.ID, WorkoutDate: date, RideID: w.Ride.ID})
			if len(out) >= limit {
				break
			}
		}
		if len(resp.Data) < pageSize {
			break
		}
	}
	return out, nil
}

// formatDate renders a unix-seconds timestamp as YYYY-MM-DD.
func formatDate(unixSec int64) string {
	return time.Unix(unixSec, 0).Format("2006-01-02")
}

// collectDiscoveries fans out /api/ride/{id}/details fetches across workouts,
// gathers liked-in-class songs, and dedupes by song id. Order matches the
// workouts' newest-first order so the first occurrence of each song carries
// the most-recent workout context.
func collectDiscoveries(c *client.Client, workouts []listedWorkout) []discovery {
	type job struct {
		idx int
		w   listedWorkout
	}
	type result struct {
		idx       int
		w         listedWorkout
		rideTitle string
		songs     []rideSong
		err       error
	}

	jobs := make(chan job)
	results := make(chan result)

	var wg sync.WaitGroup
	workers := rideFetchConcurrency
	if workers > len(workouts) {
		workers = len(workouts)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if j.w.RideID == "" {
					results <- result{idx: j.idx, w: j.w}
					continue
				}
				title, songs, err := fetchRidePlaylist(c, j.w.RideID)
				results <- result{idx: j.idx, w: j.w, rideTitle: title, songs: songs, err: err}
			}
		}()
	}
	go func() {
		for i, w := range workouts {
			jobs <- job{idx: i, w: w}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	bySong := map[string]*discovery{}
	order := []string{}
	pending := map[int]result{}
	next := 0
	for r := range results {
		pending[r.idx] = r
		for {
			cur, ok := pending[next]
			if !ok {
				break
			}
			delete(pending, next)
			next++
			if cur.err != nil {
				continue // 404s on retired rides etc. — skip silently
			}
			for _, s := range cur.songs {
				if !s.Liked {
					continue
				}
				if d, exists := bySong[s.ID]; exists {
					d.TimesPlayed++
					continue
				}
				bySong[s.ID] = &discovery{
					SongID:      s.ID,
					Title:       s.Title,
					Artists:     s.Artists,
					Album:       s.Album,
					WorkoutID:   cur.w.ID,
					WorkoutDate: cur.w.WorkoutDate,
					RideID:      cur.w.RideID,
					RideTitle:   cur.rideTitle,
					TimesPlayed: 1,
				}
				order = append(order, s.ID)
			}
		}
	}
	out := make([]discovery, 0, len(order))
	for _, id := range order {
		out = append(out, *bySong[id])
	}
	return out
}

// rideSong is the internal shape we project the playlist into. The on-the-
// wire payload has many more fields per song; agents consuming discoveries
// only care about title + artists + liked-flag + (for dedupe) song id.
type rideSong struct {
	ID      string
	Title   string
	Artists []string
	Album   string
	Liked   bool
}

func fetchRidePlaylist(c *client.Client, rideID string) (rideTitle string, songs []rideSong, err error) {
	raw, err := c.Get("/ride/"+rideID+"/details", nil)
	if err != nil {
		return "", nil, err
	}
	var resp struct {
		Ride struct {
			Title string `json:"title"`
		} `json:"ride"`
		Playlist struct {
			Songs []struct {
				ID      string `json:"id"`
				Title   string `json:"title"`
				Artists []struct {
					ArtistName string `json:"artist_name"`
				} `json:"artists"`
				Album struct {
					Name string `json:"name"`
				} `json:"album"`
				Liked bool `json:"liked"`
			} `json:"songs"`
		} `json:"playlist"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", nil, fmt.Errorf("decoding ride %s: %w", rideID, err)
	}
	out := make([]rideSong, 0, len(resp.Playlist.Songs))
	for _, s := range resp.Playlist.Songs {
		artists := make([]string, 0, len(s.Artists))
		for _, a := range s.Artists {
			if a.ArtistName != "" {
				artists = append(artists, a.ArtistName)
			}
		}
		out = append(out, rideSong{
			ID:      s.ID,
			Title:   s.Title,
			Artists: artists,
			Album:   s.Album.Name,
			Liked:   s.Liked,
		})
	}
	return resp.Ride.Title, out, nil
}

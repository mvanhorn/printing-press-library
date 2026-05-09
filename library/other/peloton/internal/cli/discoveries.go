// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/config"
)

// discovery is one in-class song the user liked, paired with the workout +
// ride context. Songs liked across multiple rides are deduped by song id;
// the discovery keeps the most-recent (first-encountered) workout context
// since the workout list is newest-first.
type discovery struct {
	SongID      string   `json:"song_id"`
	Title       string   `json:"title"`
	Artists     []string `json:"artists"`
	Album       string   `json:"album"`
	WorkoutID   string   `json:"workout_id"`
	WorkoutDate string   `json:"workout_date"`
	RideID      string   `json:"ride_id"`
	RideTitle   string   `json:"ride_title"`
	TimesPlayed int      `json:"times_played"` // how many of the scanned rides played this
}

// rideFetchConcurrency caps the parallel /api/ride/{id}/details calls. Peloton
// hasn't published a rate limit; 4 in flight has been comfortable in testing
// and matches what the SPA does when prefetching adjacent ride pages.
const rideFetchConcurrency = 4

func newDiscoveriesCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "discoveries",
		Short: "List in-class songs you liked across recent workouts",
		Long: `Scans your most-recent --limit workouts (default 30), fetches each
ride's playlist, and emits the songs where the in-class "liked" flag is
true. Songs liked in multiple rides are deduped by song id; times_played
counts how many of the scanned rides played the song.

Likes are stored Peloton-side per ride playback, not per-song globally —
this is the closest stand-in for "songs I discovered through Peloton."`,
		Example: `  peloton-pp-cli discoveries
  peloton-pp-cli discoveries --limit 100
  peloton-pp-cli discoveries --compact | jq '.[] | "\(.title) — \(.artists | join(", "))"'`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return Errf(CodeAPI, "loading config: %w", err)
			}
			if cfg.Token == "" {
				return Errf(CodeAuth, "no token saved — run `peloton-pp-cli auth login` first")
			}
			c := client.New(cfg.Token)
			if cfg.UserID == "" || strings.Contains(cfg.UserID, "|") {
				id, username, err := c.Me()
				if err != nil {
					return classify(err)
				}
				cfg.UserID = id
				cfg.Username = username
				_ = cfg.Save()
			}

			workouts, err := c.ListWorkouts(cfg.UserID, limit, nil)
			if err != nil {
				return classify(err)
			}

			discoveries, err := collectDiscoveries(c, workouts)
			if err != nil {
				return classify(err)
			}
			return emitDiscoveries(cmd, flags, discoveries)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 30, "How many recent workouts to scan")
	return cmd
}

// collectDiscoveries fans out ride-details fetches across workouts, gathers
// liked songs, and dedupes by song id. Returns a list ordered by most-recent
// workout (newest-first), which matches workouts' natural order.
//
// One auth/rate-limit failure aborts the whole walk — partial data with
// silent failures would be confusing.
func collectDiscoveries(c *client.Client, workouts []client.Workout) ([]discovery, error) {
	type job struct {
		idx     int
		workout client.Workout
	}
	type result struct {
		idx    int
		ride   client.RideDetails
		err    error
		origin client.Workout
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
				if j.workout.RideID == "" {
					results <- result{idx: j.idx, origin: j.workout}
					continue
				}
				rd, err := c.GetRideDetails(j.workout.RideID)
				results <- result{idx: j.idx, ride: rd, err: err, origin: j.workout}
			}
		}()
	}
	go func() {
		for i, w := range workouts {
			jobs <- job{idx: i, workout: w}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	bySong := map[string]*discovery{}
	order := []string{} // preserve newest-first order via first-seen workout idx
	pending := map[int]result{}
	next := 0
	var fatalErr error
	for r := range results {
		pending[r.idx] = r
		// Drain in workout order so song-first-seen tracks the newest workout.
		for {
			cur, ok := pending[next]
			if !ok {
				break
			}
			delete(pending, next)
			next++
			if cur.err != nil {
				// Auth / rate-limit are fatal; any other API error too —
				// partial discoveries silently drop liked songs the user owns.
				if fatalErr == nil {
					var ae *client.AuthError
					var re *client.RateLimitError
					if errors.As(cur.err, &ae) || errors.As(cur.err, &re) {
						fatalErr = cur.err
					} else {
						// Non-fatal per-ride errors (404 etc.) are skipped —
						// some rides genuinely no longer exist server-side.
						var ne *client.NotFoundError
						if !errors.As(cur.err, &ne) {
							fatalErr = cur.err
						}
					}
				}
				continue
			}
			for _, s := range cur.ride.Songs {
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
					WorkoutID:   cur.origin.ID,
					WorkoutDate: cur.origin.WorkoutDate,
					RideID:      cur.ride.RideID,
					RideTitle:   cur.ride.Title,
					TimesPlayed: 1,
				}
				order = append(order, s.ID)
			}
		}
	}
	if fatalErr != nil {
		return nil, fatalErr
	}
	out := make([]discovery, 0, len(order))
	for _, id := range order {
		out = append(out, *bySong[id])
	}
	return out, nil
}

func emitDiscoveries(cmd *cobra.Command, flags *rootFlags, ds []discovery) error {
	wantJSON := flags.asJSON || flags.compact || !isStdoutTTY()
	if wantJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		if !flags.compact {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(ds)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Discoveries (%d):\n", len(ds))
	for _, d := range ds {
		x := ""
		if d.TimesPlayed > 1 {
			x = fmt.Sprintf(" (×%d)", d.TimesPlayed)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s — %s%s\n", d.WorkoutDate, d.Title, joinNames(d.Artists), x)
	}
	return nil
}

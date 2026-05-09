// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"errors"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/client"
)

// rideFetchConcurrencyMCP mirrors the CLI's rideFetchConcurrency. Lives
// here as a separate const because importing internal/cli would pull in
// every cobra command — the MCP server doesn't need that surface.
const rideFetchConcurrencyMCP = 4

// discoveryMCP is the on-the-wire shape returned by the discoveries MCP
// tool. Field names match the CLI's discovery struct so consumers see
// identical responses regardless of transport.
type discoveryMCP struct {
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

// collectDiscoveriesMCP fans out ride-details fetches and collects liked
// songs, deduped by song id. Mirrors the CLI's collectDiscoveries but
// without the cobra dependency. Auth/rate-limit errors are fatal; 404
// on individual rides is skipped (retired-ride case).
func collectDiscoveriesMCP(c *client.Client, workouts []client.Workout) ([]discoveryMCP, error) {
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
	workers := rideFetchConcurrencyMCP
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

	bySong := map[string]*discoveryMCP{}
	order := []string{}
	pending := map[int]result{}
	next := 0
	var fatalErr error
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
				if fatalErr == nil && isFatalAPIError(cur.err) {
					fatalErr = cur.err
				}
				continue
			}
			for _, s := range cur.ride.Songs {
				if !s.Liked {
					continue
				}
				if d, ok := bySong[s.ID]; ok {
					d.TimesPlayed++
					continue
				}
				bySong[s.ID] = &discoveryMCP{
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
	out := make([]discoveryMCP, 0, len(order))
	for _, id := range order {
		out = append(out, *bySong[id])
	}
	return out, nil
}

// isFatalAPIError reports whether err should abort a multi-call walk.
// AuthError and RateLimitError are fatal (the rest of the walk would
// just fail the same way); anything else (including NotFoundError on
// retired rides) is treated as skippable.
func isFatalAPIError(err error) bool {
	var ae *client.AuthError
	if errors.As(err, &ae) {
		return true
	}
	var re *client.RateLimitError
	if errors.As(err, &re) {
		return true
	}
	var ne *client.NotFoundError
	if errors.As(err, &ne) {
		return false
	}
	// Generic API errors (5xx, 400, etc.) are also fatal — fail loud.
	return true
}

// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/config"
	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/store"
)

// syncSummary is what `peloton sync` emits at the end. Counts mirror the
// store rows; new_workouts and new_rides report what THIS run added so
// callers can ` -ne 0` check before doing post-sync work.
type syncSummary struct {
	NewWorkouts int           `json:"new_workouts"`
	NewRides    int           `json:"new_rides"`
	Counts      store.Counts  `json:"counts"`
	DBPath      string        `json:"db_path"`
	SyncedAt    time.Time     `json:"synced_at"`
	Elapsed     time.Duration `json:"elapsed_ns,omitempty"`
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		limit       int
		hydrateRide bool
		full        bool
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Mirror Peloton workouts (and optional ride playlists) into a local SQLite store",
		Long: `Walks /api/user/{id}/workouts newest-first, upserts new rows into the
local SQLite store, and (by default) backfills /api/ride/{id}/details for
any ride_id not already hydrated.

Incremental: stops paginating once a page is fully contained in the
already-stored ids. Use --full to ignore that early-stop and walk up to
--limit workouts regardless. Use --no-rides to skip ride hydration (the
faster path; only fetches workout list).

Database lives at ~/.local/share/peloton-pp-cli/peloton.db.`,
		Example: `  peloton-pp-cli sync                  # incremental, with rides
  peloton-pp-cli sync --no-rides       # workouts only
  peloton-pp-cli sync --full --limit 500
  peloton-pp-cli sync --json | jq .new_workouts`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			t0 := time.Now()
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

			ctx := cmd.Context()
			st, err := store.Open(ctx, "")
			if err != nil {
				return Errf(CodeAPI, "opening store: %w", err)
			}
			defer st.Close()

			known, err := st.KnownWorkoutIDs(ctx)
			if err != nil {
				return Errf(CodeAPI, "reading known workouts: %w", err)
			}
			knownArg := known
			if full {
				knownArg = nil // disable early-stop
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Fetching workouts (limit=%d, full=%v)…\n", limit, full)
			workouts, err := c.ListWorkouts(cfg.UserID, limit, knownArg)
			if err != nil {
				return classify(err)
			}
			newWorkouts, err := st.UpsertWorkouts(ctx, workouts)
			if err != nil {
				return Errf(CodeAPI, "upsert workouts: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "  fetched %d, new %d\n", len(workouts), newWorkouts)

			newRides := 0
			if hydrateRide {
				ids, err := st.RideIDsMissingDetails(ctx)
				if err != nil {
					return Errf(CodeAPI, "reading missing ride ids: %w", err)
				}
				if len(ids) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "Hydrating %d ride(s)…\n", len(ids))
					n, err := hydrateRides(ctx, cmd, c, st, ids)
					if err != nil {
						return classify(err)
					}
					newRides = n
				}
			}

			now := time.Now()
			if err := st.SetSyncedAt(ctx, "last_sync", now); err != nil {
				return Errf(CodeAPI, "recording last_sync: %w", err)
			}
			counts, err := st.Counts(ctx)
			if err != nil {
				return Errf(CodeAPI, "counts: %w", err)
			}
			summary := syncSummary{
				NewWorkouts: newWorkouts,
				NewRides:    newRides,
				Counts:      counts,
				DBPath:      st.Path(),
				SyncedAt:    now,
				Elapsed:     time.Since(t0),
			}
			return emitSyncSummary(cmd, flags, summary)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 200, "Max workouts to scan in one run (incremental still applies unless --full)")
	cmd.Flags().BoolVar(&hydrateRide, "rides", true, "Backfill ride details (playlists) for any ride_id not yet hydrated")
	cmd.Flags().BoolVar(&full, "full", false, "Disable the known-ids early-stop; walk every page up to --limit")
	return cmd
}

// hydrateRides fans out /api/ride/{id}/details fetches and writes each into
// the store. Same concurrency cap as discoveries — 4 in flight is what the
// SPA does when prefetching adjacent ride pages. Returns the count of rides
// successfully hydrated; 404s on individual rides are skipped (some get
// retired server-side).
func hydrateRides(ctx context.Context, cmd *cobra.Command, c *client.Client, st *store.Store, ids []string) (int, error) {
	const workers = rideFetchConcurrency

	type result struct {
		id  string
		rd  client.RideDetails
		err error
	}
	jobs := make(chan string)
	results := make(chan result)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				rd, err := c.GetRideDetails(id)
				results <- result{id: id, rd: rd, err: err}
			}
		}()
	}
	go func() {
		for _, id := range ids {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- id:
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	hydrated := 0
	for r := range results {
		if r.err != nil {
			var ne *client.NotFoundError
			if errors.As(r.err, &ne) {
				fmt.Fprintf(cmd.ErrOrStderr(), "  ride %s: 404, skipping\n", r.id)
				continue
			}
			// Drain remaining jobs (channel will close when goroutines exit)
			// before bubbling so we don't leak goroutines on early return.
			for range results {
			}
			return hydrated, r.err
		}
		if err := st.UpsertRideDetails(ctx, r.rd); err != nil {
			for range results {
			}
			return hydrated, fmt.Errorf("upsert ride %s: %w", r.id, err)
		}
		hydrated++
	}
	return hydrated, nil
}

func emitSyncSummary(cmd *cobra.Command, flags *rootFlags, s syncSummary) error {
	wantJSON := flags.asJSON || flags.compact || !isStdoutTTY()
	if wantJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		if !flags.compact {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(s)
	}
	fmt.Fprintf(cmd.OutOrStdout(),
		"sync ok: +%d workouts, +%d rides (totals: %d workouts, %d rides, %d songs, %d ride_songs)\n",
		s.NewWorkouts, s.NewRides, s.Counts.Workouts, s.Counts.Rides, s.Counts.Songs, s.Counts.RideSongs,
	)
	fmt.Fprintf(cmd.OutOrStdout(), "  db: %s\n", s.DBPath)
	return nil
}

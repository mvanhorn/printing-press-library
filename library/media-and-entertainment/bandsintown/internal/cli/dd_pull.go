// Novel helper command: `pull` — populate the typed dd_* tables from the API
// for every tracked artist. Distinct from the framework's `sync` (which writes
// to the generic `resources` table). Run after `track add` to hydrate the local
// store for the routing-intelligence commands.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/bandsintown/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/bandsintown/internal/dd"
)

func newPullCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		appID       string
		dateFilter  string
		sinceStaleH int
		tracked     bool
		artists     []string
		snapshot    bool
	)

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Fetch fresh artist + events into the local store",
		Long: `Fetch artists and their events from the Bandsintown REST API and write them
into the typed dd_* tables used by route / gaps / lineup / trend / sea-radar.

The default mode pulls every tracked artist (run 'track add' first). Use
--artists to pull specific names without modifying the watchlist. --since-stale
skips artists whose dd_artists.fetched_at is fresher than the given window.

Use --snapshot to also write an artist_snapshots row (drives the 'trend' command).`,
		Example: `  bandsintown-pp-cli pull --tracked --snapshot
  bandsintown-pp-cli pull --artists "Phoenix,Beach House" --date upcoming`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			// Resolve target list.
			db, err := dd.Open(cmd.Context(), resolveDBPath(dbPath))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			var targets []string
			if len(artists) > 0 {
				for _, a := range artists {
					a = strings.TrimSpace(a)
					if a != "" {
						targets = append(targets, a)
					}
				}
			}
			if tracked || len(targets) == 0 {
				rows, err := dd.ListTracked(cmd.Context(), db)
				if err != nil {
					return apiErr(err)
				}
				for _, r := range rows {
					targets = append(targets, r.Name)
				}
			}
			if len(targets) == 0 {
				return usageErr(fmt.Errorf("no targets: pass --artists or run `track add` first"))
			}

			// Resolve app_id (flag wins, else config, else env via config).
			if appID == "" {
				if cfg, _ := config.Load(flags.configPath); cfg != nil {
					appID = cfg.AppID
				}
			}
			if appID == "" {
				return authErr(fmt.Errorf("BANDSINTOWN_APP_ID not set: pass --app-id or set the env var (partner key required since 2025)"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type result struct {
				Artist        string `json:"artist"`
				Skipped       bool   `json:"skipped,omitempty"`
				SkippedReason string `json:"skipped_reason,omitempty"`
				EventsAdded   int    `json:"events_added"`
				EventsRemoved int    `json:"events_removed"`
				TrackerCount  int    `json:"tracker_count,omitempty"`
				Error         string `json:"error,omitempty"`
				SnappedAt     string `json:"snapped_at,omitempty"`
			}
			results := []result{}

			for _, name := range targets {
				r := result{Artist: name}

				// Staleness check using fetched_at.
				if sinceStaleH > 0 {
					var fetched string
					_ = db.QueryRowContext(cmd.Context(),
						`SELECT fetched_at FROM dd_artists WHERE name=?`, name).Scan(&fetched)
					if fetched != "" {
						if t, err := time.Parse("2006-01-02 15:04:05", fetched); err == nil {
							if time.Since(t) < time.Duration(sinceStaleH)*time.Hour {
								r.Skipped = true
								r.SkippedReason = fmt.Sprintf("fresh within %dh", sinceStaleH)
								results = append(results, r)
								continue
							}
						}
					}
				}

				artist, err := dd.FetchArtist(c, name, appID)
				if err != nil {
					r.Error = err.Error()
					results = append(results, r)
					continue
				}
				if err := dd.UpsertArtist(cmd.Context(), db, artist); err != nil {
					r.Error = err.Error()
					results = append(results, r)
					continue
				}
				r.TrackerCount = artist.TrackerCount

				if snapshot {
					ts, err := dd.SnapshotArtist(cmd.Context(), db, artist)
					if err != nil {
						r.Error = err.Error()
						results = append(results, r)
						continue
					}
					r.SnappedAt = ts
				}

				events, err := dd.FetchEvents(c, name, appID, dateFilter)
				if err != nil {
					r.Error = err.Error()
					results = append(results, r)
					continue
				}
				added, removed, err := dd.ReplaceEvents(cmd.Context(), db, name, events)
				if err != nil {
					r.Error = err.Error()
					results = append(results, r)
					continue
				}
				r.EventsAdded = added
				r.EventsRemoved = removed
				results = append(results, r)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"pulled":  len(targets),
					"results": results,
				})
			}
			// Human output: one line per artist
			fail := 0
			for _, r := range results {
				switch {
				case r.Error != "":
					fmt.Fprintf(cmd.OutOrStdout(), "FAIL %s: %s\n", r.Artist, r.Error)
					fail++
				case r.Skipped:
					fmt.Fprintf(cmd.OutOrStdout(), "SKIP %s: %s\n", r.Artist, r.SkippedReason)
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "OK   %s (tracker=%d, +%d events, -%d events)\n",
						r.Artist, r.TrackerCount, r.EventsAdded, r.EventsRemoved)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\npulled %d artists (%d errors)\n", len(targets), fail)
			if fail > 0 && fail == len(targets) {
				return apiErr(fmt.Errorf("all %d pulls failed", fail))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&appID, "app-id", "", "Partner application ID (falls back to BANDSINTOWN_APP_ID)")
	cmd.Flags().StringVar(&dateFilter, "date", "upcoming", "Event date filter: upcoming / past / all / YYYY-MM-DD,YYYY-MM-DD")
	cmd.Flags().IntVar(&sinceStaleH, "since-stale", 0, "Skip artists whose dd_artists row is fresher than N hours")
	cmd.Flags().BoolVar(&tracked, "tracked", false, "Pull every artist on the watchlist (default if --artists empty)")
	cmd.Flags().StringSliceVar(&artists, "artists", nil, "Comma-separated artist names (otherwise uses --tracked)")
	cmd.Flags().BoolVar(&snapshot, "snapshot", false, "Also write a dd_artist_snapshots row for trend analysis")

	// Hidden marker so the help text mentions the command's JSON shape.
	cmd.Annotations["pp:emit-json"] = `{"pulled":int,"results":[{"artist":str,"events_added":int,"events_removed":int,"tracker_count":int,"snapped_at":str?}]}`

	_ = json.Marshal // silence import if unused in future edits
	return cmd
}

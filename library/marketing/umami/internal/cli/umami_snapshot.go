// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: snapshot feeds the local per-day history tables that power
// watch, new-referrers, and pace. Run it daily (cron-friendly).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/umami/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/umami/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/umami/internal/store"
)

var snapshotMetricTypes = []string{"path", "referrer", "channel", "entry"}

// pp:data-source live
func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	var days int
	var metricDays int
	var sitesCSV string
	var dbPath string
	var timezone string
	var maxRuntime time.Duration
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Record daily traffic history locally — powers watch, new-referrers, and pace",
		Long: `Fetch per-day pageviews/sessions series and per-day top metrics (path,
referrer, channel, entry) for every website and store them in the local
SQLite history tables. Run it daily from cron; watch, new-referrers, and
pace read this history.`,
		Example:     "  umami-pp-cli snapshot --days 35\n  umami-pp-cli snapshot --sites restodom.fr --metric-days 14",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would snapshot %d days of history for all sites\n", days)
				return nil
			}
			// Bulk sync: hundreds of sequential calls. The root --timeout is a
			// per-request bound, not a whole-run bound, so snapshot uses its
			// own --max-runtime budget instead of boundCtx.
			ctx, cancel := context.WithTimeout(cmd.Context(), maxRuntime)
			defer cancel()
			if cliutil.IsDogfoodEnv() {
				if days > 3 {
					days = 3
				}
				if metricDays > 1 {
					metricDays = 1
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sites, err := fetchAllWebsites(ctx, c)
			if err != nil {
				return err
			}
			if sitesCSV != "" {
				want := map[string]bool{}
				for _, s := range strings.Split(sitesCSV, ",") {
					id, err := resolveWebsiteID(ctx, c, strings.TrimSpace(s))
					if err != nil {
						return err
					}
					want[id] = true
				}
				filtered := sites[:0]
				for _, s := range sites {
					if want[s.ID] {
						filtered = append(filtered, s)
					}
				}
				sites = filtered
				if cliutil.IsDogfoodEnv() && len(sites) > 1 {
					sites = sites[:1]
				}
			} else if cliutil.IsDogfoodEnv() && len(sites) > 1 {
				sites = sites[:1]
			}
			if dbPath == "" {
				dbPath = defaultDBPath("umami-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureUmamiHistory(ctx); err != nil {
				return err
			}
			// Mirror site identities so local-only commands (watch,
			// new-referrers) can resolve domains without the API.
			for _, site := range sites {
				data, err := json.Marshal(site)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not encode site %s for the local mirror: %v\n", site.Domain, err)
					continue
				}
				if err := db.Upsert("websites", site.ID, data); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not mirror site %s locally: %v\n", site.Domain, err)
				}
			}
			now := time.Now()
			window := periodWindow{ms(now.AddDate(0, 0, -days)), ms(now)}
			var statRows, metricRows int
			failures := make([]fetchFailure, 0)
			for _, site := range sites {
				fmt.Fprintf(cmd.ErrOrStderr(), "snapshot: %s series (%dd)\n", site.Domain, days)
				pv, sess, err := getDailySeries(ctx, c, site.ID, window, timezone)
				if err != nil {
					failures = append(failures, fetchFailure{ID: site.Domain, Error: err.Error()})
					continue
				}
				sessByDay := map[string]float64{}
				for _, p := range sess {
					sessByDay[normalizeSeriesDay(p.X)] = p.Y
				}
				for _, p := range pv {
					day := normalizeSeriesDay(p.X)
					if day == "" {
						continue
					}
					if err := db.UpsertUmamiStatsDay(ctx, nil, site.ID, day, p.Y, sessByDay[day]); err != nil {
						return err
					}
					statRows++
				}
				n, err := snapshotMetrics(ctx, c, db, site, metricDays, now, timezone)
				if err != nil {
					failures = append(failures, fetchFailure{ID: site.Domain + " (metrics)", Error: err.Error()})
					continue
				}
				metricRows += n
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d sites had failed fetches; history updated for the remaining sites\n", len(failures), len(sites))
			}
			out := map[string]any{
				"sites":          len(sites),
				"stat_days":      statRows,
				"metric_rows":    metricRows,
				"metric_days":    metricDays,
				"db":             dbPath,
				"fetch_failures": failures,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 35, "Days of daily series history to record")
	cmd.Flags().IntVar(&metricDays, "metric-days", 7, "Days of per-day metric detail (path/referrer/channel/entry) to record")
	cmd.Flags().StringVar(&sitesCSV, "sites", "", "Comma-separated website UUIDs or domains (default: all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&timezone, "timezone", "UTC", "IANA timezone for day bucketing")
	cmd.Flags().DurationVar(&maxRuntime, "max-runtime", 15*time.Minute, "Whole-run time budget for the bulk snapshot")
	return cmd
}

// snapshotMetrics records per-day top metrics for the trailing metricDays
// days, skipping site-days already present so daily cron runs stay cheap.
func snapshotMetrics(ctx context.Context, c *client.Client, db *store.Store, site umamiSite, metricDays int, now time.Time, timezone string) (int, error) {
	existing := map[string]bool{}
	rows, err := db.DB().QueryContext(ctx,
		`SELECT DISTINCT day || '|' || metric_type FROM umami_metrics_daily WHERE website_id = ?`, site.ID)
	if err == nil {
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				_ = rows.Close()
				return 0, fmt.Errorf("scanning existing metric days: %w", err)
			}
			existing[key] = true
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("iterating existing metric days: %w", err)
		}
		_ = rows.Close()
	}
	loc, locErr := time.LoadLocation(timezone)
	if locErr != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	count := 0
	for d := 1; d <= metricDays; d++ {
		day := dayStart.AddDate(0, 0, -d)
		key := dayKey(day)
		w := periodWindow{ms(day), ms(day.AddDate(0, 0, 1))}
		for _, mt := range snapshotMetricTypes {
			if existing[key+"|"+mt] {
				continue
			}
			metricRows, err := getAllMetrics(ctx, c, site.ID, mt, w)
			if err != nil {
				return count, fmt.Errorf("metrics %s day %s: %w", mt, key, err)
			}
			// Write the whole (day, metric_type) pair in one transaction so an
			// interrupted run leaves no partial rows: the existing-pair skip
			// above then correctly means "complete", never "frozen partial".
			tx, err := db.DB().BeginTx(ctx, nil)
			if err != nil {
				return count, fmt.Errorf("beginning metrics tx %s day %s: %w", mt, key, err)
			}
			wrote := 0
			for _, r := range metricRows {
				if r.X == "" {
					continue
				}
				if err := db.UpsertUmamiMetricDay(ctx, tx, site.ID, key, mt, r.X, r.Y); err != nil {
					_ = tx.Rollback()
					return count, err
				}
				wrote++
			}
			if err := tx.Commit(); err != nil {
				return count, fmt.Errorf("committing metrics %s day %s: %w", mt, key, err)
			}
			count += wrote
		}
	}
	return count, nil
}

// normalizeSeriesDay reduces a series x-value ("2026-07-01 00:00:00" or ISO)
// to its YYYY-MM-DD day key. Returns "" for unparseable values.
func normalizeSeriesDay(x string) string {
	if len(x) >= 10 {
		candidate := x[:10]
		if _, err := time.Parse("2006-01-02", candidate); err == nil {
			return candidate
		}
	}
	return ""
}

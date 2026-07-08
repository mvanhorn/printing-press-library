// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: portfolio anomaly watch. Compares each site's latest
// complete day against its trailing 28-day per-weekday baseline from the
// local snapshot history. Poll-once and cron-friendly.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/umami/internal/store"
)

// pp:data-source local
func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var threshold float64
	var dbPath string
	var timezone string
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Scan every synced site against its 28-day per-weekday baseline and report only deviations",
		Long: `Compare each site's latest complete day (yesterday, UTC) against the
average of the same weekday over the trailing 28 days of local snapshot
history, and print only the sites deviating beyond the threshold. Nothing
to report means everything is within baseline — ideal for cron.

Use this command for daily/weekly anomaly detection across synced local
history for all sites. Do NOT use this command for last-30-minutes realtime
spike checks; use 'pulse' instead.`,
		Example:     "  umami-pp-cli watch --json\n  umami-pp-cli watch --threshold 50",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--threshold=30"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan local history for baseline deviations")
				return nil
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("watch reads local snapshot history only; there is no live equivalent (run 'umami-pp-cli snapshot' to refresh history)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dbPath == "" {
				dbPath = defaultDBPath("umami-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local history at %s\nrun: umami-pp-cli snapshot --days 35 --db %s\n", dbPath, dbPath)
				out := map[string]any{"scanned_sites": 0, "deviations": []any{},
					"note": "no local history; run 'umami-pp-cli snapshot' first"}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			loc, locErr := time.LoadLocation(timezone)
			if locErr != nil {
				return usageErr(fmt.Errorf("invalid --timezone %q: %v", timezone, locErr))
			}
			names := localSiteDomains(ctx, db)
			yesterday := time.Now().In(loc).AddDate(0, 0, -1)
			day := dayKey(yesterday)
			floor := dayKey(yesterday.AddDate(0, 0, -28))
			rows, err := db.DB().QueryContext(ctx, `
				SELECT cur.website_id, cur.sessions,
				       (SELECT AVG(h.sessions) FROM umami_stats_daily h
				         WHERE h.website_id = cur.website_id
				           AND h.day < ? AND h.day >= ?
				           AND strftime('%w', h.day) = strftime('%w', ?)) AS baseline,
				       (SELECT COUNT(*) FROM umami_stats_daily h
				         WHERE h.website_id = cur.website_id
				           AND h.day < ? AND h.day >= ?
				           AND strftime('%w', h.day) = strftime('%w', ?)) AS samples
				  FROM umami_stats_daily cur
				 WHERE cur.day = ?`,
				day, floor, day, day, floor, day, day)
			if err != nil {
				if isMissingHistoryTable(err) {
					fmt.Fprintln(cmd.ErrOrStderr(), "no snapshot history yet\nrun: umami-pp-cli snapshot --days 35")
					if flags.asJSON || flags.agent {
						fmt.Fprintln(cmd.OutOrStdout(), "[]")
					}
					return nil
				}
				return fmt.Errorf("querying history: %w", err)
			}
			type deviation struct {
				Website      string  `json:"website"`
				Domain       string  `json:"domain"`
				Day          string  `json:"day"`
				Sessions     float64 `json:"sessions"`
				Baseline     float64 `json:"baseline"`
				DeviationPct float64 `json:"deviation_pct"`
				Direction    string  `json:"direction"`
				Samples      int     `json:"baseline_samples"`
			}
			scanned := 0
			deviations := make([]deviation, 0)
			for rows.Next() {
				var id string
				var sessions float64
				var baseline sql.NullFloat64
				var samples sql.NullInt64
				if err := rows.Scan(&id, &sessions, &baseline, &samples); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning history row: %w", err)
				}
				scanned++
				if !baseline.Valid || samples.Int64 < 2 || baseline.Float64 == 0 {
					continue
				}
				dev := pctChange(sessions, baseline.Float64)
				if dev >= threshold || dev <= -threshold {
					dir := "up"
					if dev < 0 {
						dir = "down"
					}
					deviations = append(deviations, deviation{
						Website: id, Domain: names[id], Day: day,
						Sessions: sessions, Baseline: round1(baseline.Float64),
						DeviationPct: round1(dev), Direction: dir, Samples: int(samples.Int64),
					})
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating history: %w", err)
			}
			_ = rows.Close()
			out := map[string]any{
				"day":           day,
				"threshold_pct": threshold,
				"scanned_sites": scanned,
				"deviations":    deviations,
			}
			if scanned == 0 {
				out["note"] = "no history rows for yesterday; run 'umami-pp-cli snapshot' to refresh"
			} else if len(deviations) == 0 {
				out["note"] = "all sites within their baselines"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 30, "Deviation percentage that triggers a report")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&timezone, "timezone", "UTC", "IANA timezone — must match the --timezone used by snapshot")
	return cmd
}

// localSiteDomains maps website IDs to domains from the synced websites mirror.
func localSiteDomains(ctx context.Context, db *store.Store) map[string]string {
	names := map[string]string{}
	rows, err := db.DB().QueryContext(ctx,
		`SELECT id, COALESCE(json_extract(data, '$.domain'), '') FROM resources WHERE resource_type = 'websites'`)
	if err != nil {
		return names
	}
	for rows.Next() {
		var id, domain string
		if err := rows.Scan(&id, &domain); err != nil {
			continue
		}
		names[id] = domain
	}
	_ = rows.Err()
	_ = rows.Close()
	return names
}

func isMissingHistoryTable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such table: umami_")
}

// resolveLocalSite resolves a domain or display name against the local
// websites mirror, mirroring resolveWebsiteID's matching (domain or name).
func resolveLocalSite(ctx context.Context, db *store.Store, arg string) string {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT id, COALESCE(json_extract(data, '$.domain'), ''), COALESCE(json_extract(data, '$.name'), '') FROM resources WHERE resource_type = 'websites'`)
	if err != nil {
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var id, domain, name string
		if err := rows.Scan(&id, &domain, &name); err != nil {
			continue
		}
		if equalDomain(domain, arg) || strings.EqualFold(name, arg) {
			return id
		}
	}
	return ""
}

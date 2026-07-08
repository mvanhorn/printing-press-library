// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: new-referrer discovery. Anti-joins the current window's
// referrers against the site's full local referrer history — referrer
// domains never seen before are new backlinks/mentions.

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/umami/internal/store"
)

// pp:data-source local
func newNovelNewReferrersCmd(flags *rootFlags) *cobra.Command {
	var flagPeriod string
	var dbPath string
	var limit int
	var timezone string
	cmd := &cobra.Command{
		Use:   "new-referrers [site]",
		Short: "Referrer domains appearing for the first time in the current window — backlink discovery",
		Long: `Anti-join the current window's referrer domains against the site's full
local referrer history: anything never seen before the window is a new
backlink or mention. Requires snapshot history (umami-pp-cli snapshot).

Use this command to list referrer domains first seen in the current window
(new backlinks/mentions). Do NOT use it for overall referrer rankings; use
'seo' or 'websites metrics' with --type referrer instead.`,
		Example:     "  umami-pp-cli new-referrers restodom.fr --period 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574;--period=7d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would anti-join current referrers against local history")
				return nil
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("new-referrers needs local referrer history; there is no live equivalent (run 'umami-pp-cli snapshot' first)"))
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site argument (UUID or domain) is required"))
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dbPath == "" {
				dbPath = defaultDBPath("umami-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local history at %s\nrun: umami-pp-cli snapshot --days 35 --metric-days 35 --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			websiteID := args[0]
			if !store.IsUUID(websiteID) {
				resolved := resolveLocalSite(ctx, db, websiteID)
				if resolved == "" {
					return notFoundErr(fmt.Errorf("no synced website matches %q; pass the UUID or run 'umami-pp-cli snapshot' first", websiteID))
				}
				websiteID = resolved
			}
			loc, locErr := time.LoadLocation(timezone)
			if locErr != nil {
				return usageErr(fmt.Errorf("invalid --timezone %q: %v", timezone, locErr))
			}
			w, err := parsePeriod(flagPeriod, time.Now().In(loc))
			if err != nil {
				return usageErr(err)
			}
			windowStart := time.UnixMilli(w.StartMs).In(loc).Format("2006-01-02")
			// Exclusive end day from the last instant INSIDE the window, so
			// midnight-aligned periods (yesterday, last-month) don't leak the
			// first day after the window.
			windowEnd := time.UnixMilli(w.EndMs-1).In(loc).AddDate(0, 0, 1).Format("2006-01-02")
			// History depth check: how many distinct referrer days exist before the window?
			var historyDays int
			if err := db.DB().QueryRowContext(ctx,
				`SELECT COUNT(DISTINCT day) FROM umami_metrics_daily
				  WHERE website_id = ? AND metric_type = 'referrer' AND day < ?`,
				websiteID, windowStart).Scan(&historyDays); err != nil {
				if isMissingHistoryTable(err) {
					fmt.Fprintln(cmd.ErrOrStderr(), "no snapshot history yet\nrun: umami-pp-cli snapshot --days 35 --metric-days 35")
					if flags.asJSON || flags.agent {
						fmt.Fprintln(cmd.OutOrStdout(), "[]")
					}
					return nil
				}
				return fmt.Errorf("checking history depth: %w", err)
			}
			rows, err := db.DB().QueryContext(ctx, `
				SELECT name, SUM(value) AS visits, MIN(day) AS first_seen
				  FROM umami_metrics_daily cur
				 WHERE website_id = ? AND metric_type = 'referrer'
				   AND day >= ? AND day < ?
				   AND name <> ''
				   AND NOT EXISTS (
				       SELECT 1 FROM umami_metrics_daily h
				        WHERE h.website_id = cur.website_id
				          AND h.metric_type = 'referrer'
				          AND h.name = cur.name
				          AND h.day < ?)
				 GROUP BY name`,
				websiteID, windowStart, windowEnd, windowStart)
			if err != nil {
				return fmt.Errorf("querying new referrers: %w", err)
			}
			type newReferrer struct {
				Referrer  string  `json:"referrer"`
				Visits    float64 `json:"visits"`
				FirstSeen string  `json:"first_seen"`
			}
			found := make([]newReferrer, 0)
			for rows.Next() {
				var r newReferrer
				if err := rows.Scan(&r.Referrer, &r.Visits, &r.FirstSeen); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning referrer row: %w", err)
				}
				found = append(found, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating referrers: %w", err)
			}
			_ = rows.Close()
			sort.Slice(found, func(i, j int) bool { return found[i].Visits > found[j].Visits })
			if len(found) > limit {
				found = found[:limit]
			}
			out := map[string]any{
				"site":          args[0],
				"website":       websiteID,
				"period":        flagPeriod,
				"history_days":  historyDays,
				"new_referrers": found,
			}
			if historyDays == 0 {
				out["note"] = "no referrer history before this window — every referrer would look new; run 'umami-pp-cli snapshot --metric-days 35' and retry after history accumulates"
				out["new_referrers"] = []newReferrer{}
			} else if len(found) == 0 {
				out["note"] = "no first-time referrers in this window"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagPeriod, "period", "7d", "Current window (compared against all prior local history)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max new referrers returned")
	cmd.Flags().StringVar(&timezone, "timezone", "UTC", "IANA timezone — must match the --timezone used by snapshot")
	return cmd
}

func equalDomain(a, b string) bool {
	trim := func(s string) string {
		return strings.TrimPrefix(strings.ToLower(s), "www.")
	}
	return trim(a) == trim(b)
}

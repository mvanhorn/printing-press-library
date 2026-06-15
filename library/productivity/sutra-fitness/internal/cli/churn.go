// Copyright 2026 adam-birddog and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored transcendence command: churn / at-risk clients.
//
// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type churnRow struct {
	ClientName    string `json:"client_name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	LastCheckIn   string `json:"last_check_in"`
	DaysInactive  int    `json:"days_inactive"`
	HasActivePlan bool   `json:"has_active_plan"`
	Reason        string `json:"reason"`
}

func newNovelChurnCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var inactiveDays int

	cmd := &cobra.Command{
		Use:   "churn",
		Short: "Flag non-removed clients with no recent check-in and/or an expired plan using a mechanical recency threshold.",
		Long: `Flag at-risk clients drifting away from your studio.

Joins your locally synced clients, reservations, and purchases to flag
non-removed clients whose last check-in is older than --inactive-days (default
30) and reports whether they still hold an active plan. Clients who joined more
recently than the threshold are excluded as too new to be lapsed. This behavioral
signal does not exist in any single Sutra endpoint.

Run 'sutra-fitness-pp-cli sync' first to populate clients, reservations, and purchases.

Use this command for behavioral at-risk clients. For hard date/credit expiry use
'expiring'.`,
		Example:     "  sutra-fitness-pp-cli churn --inactive-days 30 --json\n  sutra-fitness-pp-cli churn --inactive-days 60",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if inactiveDays <= 0 {
				inactiveDays = 30
			}

			db, ready, err := openAnalyticsStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if !ready {
				return emitAnalytics(cmd, flags, []churnRow{})
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "clients") {
				hintIfStale(cmd, db, "clients", flags.maxAge)
			}

			// Last check-in per client from reservations.
			lastCheckIn := map[string]string{}
			ciRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT json_extract(data,'$.client_id') AS cid,
				       MAX(COALESCE(json_extract(data,'$.checked_in_at'), json_extract(data,'$.updated_at'))) AS last_ci
				FROM reservations
				WHERE json_extract(data,'$.status') IN ('ATTENDED','CHECKED_IN') OR json_extract(data,'$.checked_in')=1
				GROUP BY cid`)
			if err != nil {
				return fmt.Errorf("querying reservations: %w", err)
			}
			for ciRows.Next() {
				var cid sql.NullString
				var last sql.NullString
				if err := ciRows.Scan(&cid, &last); err != nil {
					continue
				}
				if cid.Valid && cid.String != "" {
					lastCheckIn[cid.String] = trimQuotes(last.String)
				}
			}
			_ = ciRows.Close()

			// Active-plan flag per client.
			activePlan := map[string]bool{}
			apRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT client_id, COUNT(*) FROM purchases
				WHERE UPPER(COALESCE(status,''))='ACTIVE' AND client_id IS NOT NULL
				GROUP BY client_id`)
			if err != nil {
				return fmt.Errorf("querying purchases: %w", err)
			}
			for apRows.Next() {
				var cid string
				var n int
				if err := apRows.Scan(&cid, &n); err != nil {
					continue
				}
				activePlan[cid] = n > 0
			}
			_ = apRows.Close()

			now := time.Now()
			cutoff := now.AddDate(0, 0, -inactiveDays)
			clRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, COALESCE(first_name,''), COALESCE(last_name,''), COALESCE(email,''), COALESCE(phone,''), COALESCE(created_at,'')
				FROM clients
				WHERE COALESCE(removed,0)=0`)
			if err != nil {
				return fmt.Errorf("querying clients: %w", err)
			}
			defer clRows.Close()

			out := make([]churnRow, 0)
			for clRows.Next() {
				var id, first, last, email, phone, created string
				if err := clRows.Scan(&id, &first, &last, &email, &phone, &created); err != nil {
					continue
				}
				// Skip clients too new to be considered lapsed.
				if createdTime, ok := parseLocalTime(created); ok && createdTime.After(cutoff) {
					continue
				}

				lastCI := lastCheckIn[id]
				lastTime, hasCI := parseLocalTime(lastCI)
				if hasCI && lastTime.After(cutoff) {
					continue // active recently — not at risk
				}

				reasons := []string{}
				if !hasCI {
					reasons = append(reasons, "no_check_in_on_record")
				} else {
					reasons = append(reasons, "no_recent_check_in")
				}
				hasPlan := activePlan[id]
				if !hasPlan {
					reasons = append(reasons, "no_active_plan")
				}

				daysInactive := -1
				if hasCI {
					daysInactive = int(now.Sub(lastTime).Hours() / 24)
				}

				out = append(out, churnRow{
					ClientName:    strings.TrimSpace(first + " " + last),
					Email:         email,
					Phone:         phone,
					LastCheckIn:   lastCI,
					DaysInactive:  daysInactive,
					HasActivePlan: hasPlan,
					Reason:        strings.Join(reasons, "+"),
				})
			}
			if err := clRows.Err(); err != nil {
				return fmt.Errorf("iterating clients: %w", err)
			}
			sort.SliceStable(out, func(i, j int) bool {
				// DaysInactive == -1 means no check-in on record — the most
				// at-risk clients — so rank them above any finite inactivity.
				di, dj := out[i].DaysInactive, out[j].DaysInactive
				if di < 0 {
					di = int(^uint(0) >> 1)
				}
				if dj < 0 {
					dj = int(^uint(0) >> 1)
				}
				return di > dj
			})

			return emitAnalytics(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&inactiveDays, "inactive-days", 30, "Days without a check-in before a client is flagged at-risk")
	return cmd
}

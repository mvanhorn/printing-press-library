// Copyright 2026 adam-birddog and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored transcendence command: no-show rates.
//
// pp:data-source local
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type noShowRow struct {
	Key        string  `json:"key"`
	NoShows    int     `json:"no_shows"`
	CheckedIn  int     `json:"checked_in"`
	Resolved   int     `json:"resolved"`
	NoShowRate float64 `json:"no_show_rate"`
}

func newNovelNoShowsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var groupBy string
	var limit int

	cmd := &cobra.Command{
		Use:   "no-shows",
		Short: "Surface no-show rates grouped by instructor, class, or client from synced reservations.",
		Long: `Surface no-show rates from your locally synced reservations.

A no-show is a reservation still marked BOOKED for a class that has already
started (never marked attended); attendance is the ATTENDED status. The rate is
no-shows over resolved outcomes (attended + no-show), grouped by instructor,
class, or client. The Sutra dashboard buries this inside a fixed Attendance
report.

Run 'sutra-fitness-pp-cli sync' first to populate classes and reservations.

Use this command to pivot no-shows by instructor, class, or client. For a full
instructor ranking use 'scorecard'; for renewal-risk clients use 'churn'.`,
		Example:     "  sutra-fitness-pp-cli no-shows --group-by instructor\n  sutra-fitness-pp-cli no-shows --group-by client --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			groupBy = strings.ToLower(strings.TrimSpace(groupBy))
			if groupBy == "" {
				groupBy = "instructor"
			}

			var keyExpr string
			switch groupBy {
			case "instructor":
				keyExpr = "COALESCE(NULLIF(c.instructor_name,''),'(unassigned)')"
			case "class":
				keyExpr = "COALESCE(NULLIF(c.name,''), r.classes_id)"
			case "client":
				keyExpr = "COALESCE(NULLIF(TRIM(COALESCE(json_extract(r.data,'$.client.first_name'),'')||' '||COALESCE(json_extract(r.data,'$.client.last_name'),'')),''), json_extract(r.data,'$.client_id'), '(unknown)')"
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--group-by must be one of: instructor, class, client"))
			}

			db, ready, err := openAnalyticsStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if !ready {
				return emitAnalytics(cmd, flags, []noShowRow{})
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "reservations") {
				hintIfStale(cmd, db, "reservations", flags.maxAge)
			}

			// Real Sutra data has no NO_SHOW status: a no-show is a reservation
			// still 'BOOKED' for a class that has already started and was not
			// attended. Attendance is status 'ATTENDED' (equiv. the checked_in
			// flag). Spec values CHECKED_IN/NO_SHOW are also accepted.
			query := fmt.Sprintf(`
				SELECT %s AS grp,
				       SUM(CASE WHEN json_extract(r.data,'$.status')='NO_SHOW'
				                  OR (json_extract(r.data,'$.status')='BOOKED'
				                      AND COALESCE(json_extract(r.data,'$.checked_in'),0)<>1
				                      AND c.start_time < strftime('%%Y-%%m-%%dT%%H:%%M:%%SZ','now')) THEN 1 ELSE 0 END) AS no_shows,
				       SUM(CASE WHEN json_extract(r.data,'$.checked_in')=1 OR json_extract(r.data,'$.status') IN ('ATTENDED','CHECKED_IN') THEN 1 ELSE 0 END) AS checked_in
				FROM reservations r
				JOIN classes c ON r.classes_id = c.id
				WHERE COALESCE(c.canceled,0)=0 AND COALESCE(c.deleted,0)=0
				GROUP BY grp`, keyExpr)

			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("querying reservations: %w", err)
			}
			defer rows.Close()

			out := make([]noShowRow, 0)
			for rows.Next() {
				var key string
				var noShows, checkedIn int
				if err := rows.Scan(&key, &noShows, &checkedIn); err != nil {
					continue
				}
				resolved := noShows + checkedIn
				out = append(out, noShowRow{
					Key:        key,
					NoShows:    noShows,
					CheckedIn:  checkedIn,
					Resolved:   resolved,
					NoShowRate: pct(noShows, resolved),
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating reservations: %w", err)
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].NoShowRate != out[j].NoShowRate {
					return out[i].NoShowRate > out[j].NoShowRate
				}
				return out[i].NoShows > out[j].NoShows
			})
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			return emitAnalytics(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&groupBy, "group-by", "instructor", "Group by: instructor, class, or client")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	return cmd
}

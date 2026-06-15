// Copyright 2026 adam-birddog and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored transcendence command: instructor scorecard.
//
// pp:data-source local
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type instructorScore struct {
	Name          string  `json:"name"`
	ClassesTaught int     `json:"classes_taught"`
	Capacity      int     `json:"capacity"`
	Booked        int     `json:"booked"`
	FillRate      float64 `json:"fill_rate"`
	Reservations  int     `json:"reservations"`
	CheckedIn     int     `json:"checked_in"`
	NoShows       int     `json:"no_shows"`
	NoShowRate    float64 `json:"no_show_rate"`
	CheckInRate   float64 `json:"check_in_rate"`
}

type scorecardView struct {
	Instructors []instructorScore `json:"instructors"`
}

func newNovelScorecardCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "scorecard",
		Short: "Rank instructors by class fill rate, no-show rate, and check-in rate across your synced schedule.",
		Long: `Rank instructors by class fill rate, no-show rate, and check-in rate.

Joins your locally synced classes and reservations to score each instructor on
fill (booked/capacity), no-show rate, and check-in rate. The Sutra API has no
per-instructor reporting endpoint; this report exists only over the local mirror.

Run 'sutra-fitness-pp-cli sync' first to populate classes and reservations.

Use this command to rank instructor performance. For per-slot capacity use
'utilization'; to pivot no-shows by class or client use 'no-shows'.`,
		Example:     "  sutra-fitness-pp-cli scorecard\n  sutra-fitness-pp-cli scorecard --agent --select instructors.name,instructors.fill_rate,instructors.no_show_rate",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, ready, err := openAnalyticsStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if !ready {
				return emitAnalytics(cmd, flags, scorecardView{Instructors: []instructorScore{}})
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "classes") {
				hintIfStale(cmd, db, "classes", flags.maxAge)
			}

			scores := map[string]*instructorScore{}
			order := []string{}
			get := func(name string) *instructorScore {
				if s, ok := scores[name]; ok {
					return s
				}
				s := &instructorScore{Name: name}
				scores[name] = s
				order = append(order, name)
				return s
			}

			// Fill rate from the classes table.
			fillRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(NULLIF(instructor_name,''),'(unassigned)') AS instructor,
				       COUNT(*) AS classes,
				       COALESCE(SUM(max_capacity),0) AS capacity,
				       COALESCE(SUM(total_booked),0) AS booked
				FROM classes
				WHERE COALESCE(canceled,0)=0 AND COALESCE(deleted,0)=0
				GROUP BY instructor`)
			if err != nil {
				return fmt.Errorf("querying classes: %w", err)
			}
			for fillRows.Next() {
				var name string
				var classes, capacity, booked int
				if err := fillRows.Scan(&name, &classes, &capacity, &booked); err != nil {
					continue
				}
				s := get(name)
				s.ClassesTaught = classes
				s.Capacity = capacity
				s.Booked = booked
				s.FillRate = pct(booked, capacity)
			}
			_ = fillRows.Close()
			if err := fillRows.Err(); err != nil {
				return fmt.Errorf("iterating classes: %w", err)
			}

			// Attendance outcomes from reservations joined to their class.
			// Real Sutra data marks attendance as status 'ATTENDED' (equiv. the
			// checked_in flag); there is no NO_SHOW status, so a no-show is a
			// reservation still 'BOOKED' for a class that has already started.
			// We also accept the spec's CHECKED_IN/NO_SHOW values for robustness.
			resRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(NULLIF(c.instructor_name,''),'(unassigned)') AS instructor,
				       COUNT(*) AS reservations,
				       SUM(CASE WHEN json_extract(r.data,'$.checked_in')=1 OR json_extract(r.data,'$.status') IN ('ATTENDED','CHECKED_IN') THEN 1 ELSE 0 END) AS checked_in,
				       SUM(CASE WHEN json_extract(r.data,'$.status')='NO_SHOW'
				                  OR (json_extract(r.data,'$.status')='BOOKED'
				                      AND COALESCE(json_extract(r.data,'$.checked_in'),0)<>1
				                      AND c.start_time < strftime('%Y-%m-%dT%H:%M:%SZ','now')) THEN 1 ELSE 0 END) AS no_shows
				FROM reservations r
				JOIN classes c ON r.classes_id = c.id
				WHERE COALESCE(c.canceled,0)=0 AND COALESCE(c.deleted,0)=0
				GROUP BY instructor`)
			if err != nil {
				return fmt.Errorf("querying reservations: %w", err)
			}
			for resRows.Next() {
				var name string
				var reservations, checkedIn, noShows int
				if err := resRows.Scan(&name, &reservations, &checkedIn, &noShows); err != nil {
					continue
				}
				s := get(name)
				s.Reservations = reservations
				s.CheckedIn = checkedIn
				s.NoShows = noShows
				resolved := checkedIn + noShows
				s.NoShowRate = pct(noShows, resolved)
				s.CheckInRate = pct(checkedIn, resolved)
			}
			_ = resRows.Close()
			if err := resRows.Err(); err != nil {
				return fmt.Errorf("iterating reservations: %w", err)
			}

			out := make([]instructorScore, 0, len(order))
			for _, name := range order {
				out = append(out, *scores[name])
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].FillRate != out[j].FillRate {
					return out[i].FillRate > out[j].FillRate
				}
				return out[i].Name < out[j].Name
			})
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			return emitAnalytics(cmd, flags, scorecardView{Instructors: out})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum instructors to return (0 = all)")
	return cmd
}

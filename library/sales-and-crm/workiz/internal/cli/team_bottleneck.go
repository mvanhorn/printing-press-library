// Copyright 2026 Eldar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type bottleneckConflict struct {
	Type    string `json:"type"`
	Detail  string `json:"detail"`
	JobUUID string `json:"job_uuid,omitempty"`
}

type bottleneckCrew struct {
	Member         string               `json:"member"`
	ScheduledJobs  int                  `json:"scheduled_jobs"`
	ScheduledHours float64              `json:"scheduled_hours"`
	Conflicts      []bottleneckConflict `json:"conflicts"`
}

func newNovelTeamBottleneckCmd(flags *rootFlags) *cobra.Command {
	var flagWeek bool
	var dbPath string

	cmd := &cobra.Command{
		Use:         "bottleneck",
		Short:       "See per-crew scheduled load and catch double-bookings or time-off conflicts before they become no-shows.",
		Long:        "Use this for aggregate crew load AND itemized double-booking/time-off conflicts in one pass. Do not look for a separate \"conflicts\" or \"available\" command — this subsumes both.",
		Example:     "  workiz-pp-cli team bottleneck --week --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute per-crew scheduled load and conflicts from the local mirror")
				return nil
			}
			ctx := cmd.Context()
			var bail bool
			if dbPath, bail = checkNovelMirror(cmd, flags, dbPath, "job,team,timeoff", []bottleneckCrew{}); bail {
				return nil
			}
			db, err := openNovelStore(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			jobs, err := loadJobs(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("loading jobs: %w", err)
			}
			timeoffs, err := loadTimeOff(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("loading time off: %w", err)
			}

			now := time.Now()
			windowStart, windowEnd := now, now.AddDate(0, 0, 7)
			if flagWeek {
				weekday := int(now.Weekday())
				if weekday == 0 {
					weekday = 7 // ISO: Sunday = 7
				}
				monday := now.AddDate(0, 0, -(weekday - 1))
				windowStart = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, monday.Location())
				windowEnd = windowStart.AddDate(0, 0, 7)
			}

			type scheduledJob struct {
				uuid  string
				start time.Time
				end   time.Time
			}
			byMember := map[string][]scheduledJob{}
			for _, j := range jobs {
				start, ok := parseWorkizTime(j.JobDateTime)
				if !ok || start.Before(windowStart) || start.After(windowEnd) {
					continue
				}
				end, hasEnd := parseWorkizTime(j.JobEndDateTime)
				if !hasEnd {
					end = start.Add(2 * time.Hour) // Workiz jobs without an explicit end are short-duration by convention; 2h is a conservative default for conflict detection
				}
				for _, member := range j.Team {
					if member.Name == "" {
						continue
					}
					byMember[member.Name] = append(byMember[member.Name], scheduledJob{uuid: j.UUID, start: start, end: end})
				}
			}

			// timeoff by member name (Workiz keys time-off by username)
			offByMember := map[string][]wzTimeOff{}
			for _, t := range timeoffs {
				if t.UserName == "" {
					continue
				}
				offByMember[t.UserName] = append(offByMember[t.UserName], t)
			}

			members := make([]string, 0, len(byMember))
			for m := range byMember {
				members = append(members, m)
			}
			sort.Strings(members)

			results := make([]bottleneckCrew, 0, len(members))
			for _, member := range members {
				scheduled := byMember[member]
				sort.Slice(scheduled, func(i, j int) bool { return scheduled[i].start.Before(scheduled[j].start) })

				var totalHours float64
				conflicts := make([]bottleneckConflict, 0)
				for i, s := range scheduled {
					totalHours += s.end.Sub(s.start).Hours()
					// Overlapping-booking conflict: this job's window overlaps the next one for the same crew member.
					if i+1 < len(scheduled) {
						next := scheduled[i+1]
						if next.start.Before(s.end) {
							conflicts = append(conflicts, bottleneckConflict{
								Type:    "double_booking",
								Detail:  fmt.Sprintf("job %s overlaps job %s", s.uuid, next.uuid),
								JobUUID: s.uuid,
							})
						}
					}
					// Time-off conflict: crew assigned during their own recorded time off.
					for _, off := range offByMember[member] {
						offStart, okStart := parseWorkizTime(off.StartDate)
						offEnd, okEnd := parseWorkizTime(off.EndDate)
						if !okStart || !okEnd {
							continue
						}
						if s.start.Before(offEnd) && offStart.Before(s.end) {
							conflicts = append(conflicts, bottleneckConflict{
								Type:    "time_off_conflict",
								Detail:  fmt.Sprintf("job %s scheduled during recorded time off (%s to %s)", s.uuid, off.StartDate, off.EndDate),
								JobUUID: s.uuid,
							})
						}
					}
				}

				results = append(results, bottleneckCrew{
					Member:         member,
					ScheduledJobs:  len(scheduled),
					ScheduledHours: totalHours,
					Conflicts:      conflicts,
				})
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d jobs\t%.1fh", r.Member, r.ScheduledJobs, r.ScheduledHours)
				if len(r.Conflicts) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "\t%d conflict(s)", len(r.Conflicts))
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagWeek, "week", false, "Restrict to the current calendar week (Monday-Sunday) instead of the rolling next 7 days")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/workiz-pp-cli/data.db)")
	return cmd
}

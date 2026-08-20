// Hand-built (Phase 3): recurring-drift detector. For each recurring-series
// master, compare its instances/exceptions to the master pattern and emit
// the divergent ones.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-calendar/internal/store"

	"github.com/spf13/cobra"
)

func newRecurringDriftCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "recurring-drift",
		Short: "List recurring-event instances that diverge from their series master (rescheduled, retitled, or relocated)",
		Long: `Recurring-drift compares each occurrence/exception in the local store to its
seriesMaster and reports the ones whose start/end/subject/location have
been edited away from the master pattern. These are the silent
organizer-side reschedules that cause people to join calls at the wrong hour.`,
		Example:     "  outlook-calendar-pp-cli recurring-drift --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("outlook-calendar-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			events, err := loadEvents(cmd.Context(), db.DB(), time.Time{}, time.Time{})
			if err != nil {
				return apiErr(err)
			}

			masters := map[string]graphEvent{}
			var instances []graphEvent
			for _, ev := range events {
				switch ev.Type {
				case "seriesMaster":
					masters[ev.ID] = ev
				case "occurrence", "exception":
					instances = append(instances, ev)
				}
			}

			type drift struct {
				InstanceID            string   `json:"instance_id"`
				MasterID              string   `json:"master_id"`
				MasterSubject         string   `json:"master_subject"`
				Type                  string   `json:"type"` // occurrence | exception
				Start                 string   `json:"start"`
				End                   string   `json:"end"`
				Reasons               []string `json:"reasons"`
				DeltaMinutes          int      `json:"delta_minutes,omitempty"`
				MasterLocation        string   `json:"master_location,omitempty"`
				InstanceLocation      string   `json:"instance_location,omitempty"`
				MasterSubjectMismatch string   `json:"master_subject_mismatch,omitempty"`
				InstanceSubject       string   `json:"instance_subject,omitempty"`
			}

			drifts := []drift{}
			for _, inst := range instances {
				master, ok := masters[inst.SeriesMasterID]
				if !ok {
					continue
				}
				var reasons []string

				if inst.Subject != "" && inst.Subject != master.Subject {
					reasons = append(reasons, "subject_diverged")
				}
				if inst.Location != "" && inst.Location != master.Location {
					reasons = append(reasons, "location_diverged")
				}

				deltaMins := 0
				masterStart, mErr := parseGraphTime(master.Start)
				instStart, iErr := parseGraphTime(inst.Start)
				if mErr == nil && iErr == nil {
					// Project master's time-of-day onto the instance's date and compare.
					projected := time.Date(instStart.Year(), instStart.Month(), instStart.Day(),
						masterStart.Hour(), masterStart.Minute(), masterStart.Second(), 0, instStart.Location())
					diff := instStart.Sub(projected)
					if diff < 0 {
						diff = -diff
					}
					if diff > time.Minute {
						reasons = append(reasons, "start_time_shifted")
						deltaMins = int(diff.Minutes())
					}
				}

				// PATCH: parallel end-time divergence check so silently shortened/extended instances surface alongside start-time drift.
				masterEnd, meErr := parseGraphTime(master.End)
				instEnd, ieErr := parseGraphTime(inst.End)
				if meErr == nil && ieErr == nil {
					// Same projection trick for end-time: catches durations that were
					// silently shortened or extended (start unchanged, end moved).
					projected := time.Date(instEnd.Year(), instEnd.Month(), instEnd.Day(),
						masterEnd.Hour(), masterEnd.Minute(), masterEnd.Second(), 0, instEnd.Location())
					diff := instEnd.Sub(projected)
					if diff < 0 {
						diff = -diff
					}
					if diff > time.Minute {
						reasons = append(reasons, "end_time_shifted")
						if m := int(diff.Minutes()); m > deltaMins {
							deltaMins = m
						}
					}
				}

				if len(reasons) == 0 {
					continue
				}
				drifts = append(drifts, drift{
					InstanceID:            inst.ID,
					MasterID:              master.ID,
					MasterSubject:         master.Subject,
					Type:                  inst.Type,
					Start:                 inst.Start.DateTime,
					End:                   inst.End.DateTime,
					Reasons:               reasons,
					DeltaMinutes:          deltaMins,
					MasterLocation:        master.Location,
					InstanceLocation:      inst.Location,
					MasterSubjectMismatch: master.Subject,
					InstanceSubject:       inst.Subject,
				})
			}
			sort.Slice(drifts, func(i, j int) bool { return drifts[i].Start < drifts[j].Start })

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				return printJSONFiltered(w, drifts, flags)
			}
			if len(drifts) == 0 {
				fmt.Fprintln(w, "No recurring-event drift detected.")
				return nil
			}
			fmt.Fprintf(w, "%d drifted instance(s):\n", len(drifts))
			for _, d := range drifts {
				fmt.Fprintf(w, "  %s  [%s] %s — %s\n", d.Start, strings.Join(d.Reasons, ","), d.MasterSubject, d.Type)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

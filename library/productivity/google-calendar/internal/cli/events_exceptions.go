// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: recurring-event deviations in a window. Hand-implemented;
// classification lives in internal/verdict.

package cli

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/verdict"
	"github.com/spf13/cobra"
)

// exceptionRow is one deviated recurring-event instance.
type exceptionRow struct {
	Account          string `json:"account"`
	Calendar         string `json:"calendar"`
	ID               string `json:"id"`
	RecurringEventID string `json:"recurring_event_id"`
	Summary          string `json:"summary"`
	Kind             string `json:"kind"`
	Status           string `json:"status"`
	OriginalStart    string `json:"original_start"`
	Start            string `json:"start"`
	End              string `json:"end"`
}

type exceptionsOutput struct {
	Exceptions []exceptionRow   `json:"exceptions"`
	Coverage   verdict.Coverage `json:"coverage"`
}

// pp:data-source live
func newNovelEventsExceptionsCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string

	cmd := &cobra.Command{
		Use:   "exceptions",
		Short: "Recurring-event instances that moved or were cancelled in a window — routine deviations, isolated",
		Long: `Scans every calendar in calendars.yaml (singleEvents=true,
showDeleted=true) for instances that deviate from their recurring series:

  moved              originalStartTime differs from the instance's start
  cancelled_instance status "cancelled" on an instance of a recurring event

An instance sitting exactly where its rule put it is not an exception.
Both times are reported (original_start and start) so the deviation is
visible at a glance.

Carries the standard coverage block: exit 4 when any manifest calendar
could not be read, exit 0 otherwise.`,
		Example: `  google-calendar-pp-cli events exceptions --from 2026-08-18 --to 2026-08-25
  google-calendar-pp-cli events exceptions --from 2026-08-18 --to 2026-09-01 --json
  google-calendar-pp-cli events exceptions --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			from, to, err := resolveWindow(flagFrom, flagTo)
			if err != nil {
				return err
			}
			vc, err := loadVerdictContext(flags)
			if err != nil {
				return err
			}
			events, sources := fetchManifestEvents(cmd.Context(), vc, map[string]string{
				"singleEvents": "true",
				"showDeleted":  "true",
				"timeMin":      from.Format(time.RFC3339),
				"timeMax":      to.Format(time.RFC3339),
			})
			out := exceptionsOutput{
				Exceptions: []exceptionRow{},
				Coverage:   verdict.BuildCoverage(sources),
			}
			for _, e := range events {
				kind, is := verdict.ClassifyException(e)
				if !is {
					continue
				}
				ref := e.Ref()
				row := exceptionRow{
					Account:          e.Account,
					Calendar:         e.Calendar,
					ID:               e.ID,
					RecurringEventID: e.RecurringEventID,
					Summary:          e.Summary,
					Kind:             kind,
					Status:           e.Status,
					Start:            ref.Start,
					End:              ref.End,
				}
				if e.OriginalStart != nil {
					row.OriginalStart = e.OriginalStart.UTC().Format(time.RFC3339)
				}
				out.Exceptions = append(out.Exceptions, row)
			}
			sort.SliceStable(out.Exceptions, func(i, j int) bool {
				if out.Exceptions[i].OriginalStart != out.Exceptions[j].OriginalStart {
					return out.Exceptions[i].OriginalStart < out.Exceptions[j].OriginalStart
				}
				return out.Exceptions[i].ID < out.Exceptions[j].ID
			})
			if err := emitVerdict(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "recurring-event exceptions %s → %s: %d\n", from.Format(time.RFC3339), to.Format(time.RFC3339), len(out.Exceptions))
				for _, r := range out.Exceptions {
					switch r.Kind {
					case verdict.ExceptionMoved:
						fmt.Fprintf(w, "moved      %s/%s %q: %s → %s\n", r.Account, r.Calendar, r.Summary, r.OriginalStart, r.Start)
					default:
						fmt.Fprintf(w, "cancelled  %s/%s series %s instance originally at %s\n", r.Account, r.Calendar, r.RecurringEventID, r.OriginalStart)
					}
				}
				fmt.Fprintln(w, coverageSummary(out.Coverage))
				coverageErrorLines(w, out.Coverage)
			}); err != nil {
				return err
			}
			if !out.Coverage.Complete {
				return exitDegraded("coverage incomplete (%d/%d calendars read) — deviations may be missing", out.Coverage.Checked, out.Coverage.Of)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Window start: YYYY-MM-DD (local midnight) or RFC3339 (default: today)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Window end: YYYY-MM-DD (local midnight) or RFC3339 (default: --from +7 days)")
	return cmd
}

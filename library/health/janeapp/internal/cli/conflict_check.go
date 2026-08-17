// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: before booking, warn if a candidate slot collides with an
// appointment you already have — including at a *different* clinic. Only a
// unified local view across every logged-in clinic can catch cross-clinic
// double-bookings, which no single Jane portal can.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newNovelConflictCheckCmd(flags *rootFlags) *cobra.Command {
	var atStr string
	var duration int

	cmd := &cobra.Command{
		Use:   "conflict-check",
		Short: "Before booking, warn if a candidate slot collides with an existing appointment at another clinic.",
		Long: `Check a proposed appointment time against everything you already have
booked across every logged-in Jane clinic. Reports any overlap so you don't
double-book yourself between two different clinics.`,
		Example:     "  janeapp-pp-cli conflict-check --at 2026-07-15T09:00:00 --duration 60",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			if atStr == "" {
				return usageErr(fmt.Errorf("required flag --at not set (e.g. 2026-07-15T09:00:00)"))
			}
			at, err := parseFlexibleDate(atStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --at %q: use RFC3339 or YYYY-MM-DDTHH:MM:SS", atStr))
			}
			if duration <= 0 {
				return usageErr(fmt.Errorf("--duration must be a positive number of minutes"))
			}
			end := at.Add(time.Duration(duration) * time.Minute)

			clinics, err := loggedInClinics()
			if err != nil {
				return err
			}
			if len(clinics) == 0 {
				return usageErr(fmt.Errorf("no logged-in clinics to check against; run 'auth login' first"))
			}
			recs, err := gatherAppointments(cmd, flags, clinics)
			if err != nil {
				return err
			}
			var conflicts []map[string]any
			for _, r := range recs {
				if r.Start.IsZero() {
					continue
				}
				rEnd, _ := extractTimeFromKeys(r.Raw, apptEndKeys)
				if rEnd.IsZero() {
					rEnd = r.Start.Add(60 * time.Minute) // assume 60m when unknown
				}
				if r.Start.Before(end) && at.Before(rEnd) {
					conflicts = append(conflicts, r.view)
				}
			}
			result := map[string]any{
				"candidate_start": at.Format(time.RFC3339),
				"candidate_end":   end.Format(time.RFC3339),
				"conflict":        len(conflicts) > 0,
				"conflicts":       conflicts,
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				b, _ := json.Marshal(result)
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			if len(conflicts) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No conflict: %s is clear across %d clinic(s).\n", atStr, len(clinics))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "CONFLICT: %d existing appointment(s) overlap %s:\n", len(conflicts), atStr)
			for _, c := range conflicts {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s %s at %s (%s)\n", str(c["date"]), str(c["start_at"]), str(c["clinic"]), str(c["treatment"]))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&atStr, "at", "", "Proposed appointment start (RFC3339 or YYYY-MM-DDTHH:MM:SS)")
	cmd.Flags().IntVar(&duration, "duration", 60, "Proposed appointment duration in minutes")
	return cmd
}

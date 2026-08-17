// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: watch availability for a slot earlier than a target date —
// the classic "did a cancellation free up something sooner?" check. A single
// pass by default; --poll turns it into a bounded polling loop.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var treatment, staff, location, maxChecks int
	var beforeStr string
	var poll time.Duration

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Poll availability and alert when an earlier slot than a target opens up.",
		Long: `Check whether an opening exists before a target date for a given
practitioner and treatment — useful for catching a cancellation that frees up an
earlier appointment than the one you have.

Runs a single check by default. Pass --poll <duration> to keep checking on an
interval (bounded by --max-checks) until an earlier slot appears.`,
		Example:     "  janeapp-pp-cli watch --clinic embophysio --treatment 1 --staff 1 --before 2026-08-01\n  janeapp-pp-cli watch --treatment 1 --staff 1 --before 2026-08-01 --poll 10m --max-checks 24",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			if treatment <= 0 {
				return usageErr(fmt.Errorf("required flag --treatment not set (see 'treatments')"))
			}
			if staff <= 0 {
				return usageErr(fmt.Errorf("required flag --staff not set (see 'staff')"))
			}
			if beforeStr == "" {
				return usageErr(fmt.Errorf("required flag --before not set (YYYY-MM-DD)"))
			}
			before, err := parseFlexibleDate(beforeStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --before %q: use YYYY-MM-DD or RFC3339", beforeStr))
			}
			locID, err := resolveLocationID(cmd, flags, location)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now()
			horizon := int(before.Sub(now).Hours()/24) + 1
			if horizon < 1 {
				return usageErr(fmt.Errorf("--before %s is in the past", beforeStr))
			}
			checks := 1
			if poll > 0 && maxChecks > 0 {
				checks = maxChecks
			}
			for i := 0; i < checks; i++ {
				op, err := findNextOpening(cmd.Context(), c, locID, treatment, staff, now, horizon)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if op != nil {
					if t, ok := parseOpeningTime(op.StartAt); ok && t.Before(before) {
						result := map[string]any{
							"found": true, "start_at": op.StartAt, "end_at": op.EndAt,
							"duration": op.Duration, "staff_member_id": op.StaffMemberID,
							"treatment_id": op.TreatmentID, "location_id": op.LocationID,
						}
						if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
							b, _ := json.Marshal(result)
							return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
						}
						fmt.Fprintf(cmd.OutOrStdout(), "Earlier slot available: %s (before %s)\n", op.StartAt, before.Format("2006-01-02"))
						return nil
					}
				}
				if poll > 0 && i < checks-1 {
					fmt.Fprintf(cmd.ErrOrStderr(), "no earlier slot yet; next check in %s (%d/%d)\n", poll, i+1, checks)
					select {
					case <-cmd.Context().Done():
						return cmd.Context().Err()
					case <-time.After(poll):
					}
				}
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"found": false, "before": before.Format(time.RFC3339)}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "No opening before %s.\n", before.Format("2006-01-02"))
			return nil
		},
	}
	cmd.Flags().IntVar(&treatment, "treatment", 0, "Treatment ID (see 'treatments')")
	cmd.Flags().IntVar(&staff, "staff", 0, "Practitioner ID (see 'staff')")
	cmd.Flags().IntVar(&location, "location", 0, "Location ID (auto-resolved when the clinic has one location)")
	cmd.Flags().StringVar(&beforeStr, "before", "", "Target date; alert if a slot opens before this (YYYY-MM-DD)")
	cmd.Flags().DurationVar(&poll, "poll", 0, "Poll on this interval instead of checking once (e.g. 10m)")
	cmd.Flags().IntVar(&maxChecks, "max-checks", 12, "Maximum checks when --poll is set")
	return cmd
}

// parseFlexibleDate accepts YYYY-MM-DD or RFC3339.
func parseFlexibleDate(s string) (time.Time, error) {
	// RFC3339 carries its own offset, so parse it as-is. Layouts without a zone
	// are user-entered wall-clock times; anchor them to the machine's local
	// zone (not UTC) so a candidate like "2026-07-15T09:00:00" compares
	// correctly against Jane's absolute appointment times — otherwise
	// conflict-check would shift by the local offset and miss a real overlap.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q", s)
}

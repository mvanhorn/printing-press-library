// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// plan: back-solve the daily input you need to reach an hours target or a
// roadmap level by a date. Absorbs DSToolbox Get-DSGoalRequiredAverage, beaten
// with offline store-backed pace comparison. Works offline.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newPlanCmd(flags *rootFlags) *cobra.Command {
	var target string
	var by string
	var paceDays int

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Back-solve the daily minutes needed to hit an hours target or level by a date",
		Long: "Given a target (hours like 600h, or a roadmap level like 5 / L5) and a date,\n" +
			"compute the daily input you need from now, and compare it to your recent\n" +
			"pace. Works offline against the synced store.",
		Example: strings.Trim(`
  dreaming-pp-cli plan --target 600h --by 2026-12-31
  dreaming-pp-cli plan --target 5 --by 2027-06-01 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			targetHours, terr := parseTargetHours(target)
			var byDate time.Time
			var derr error
			if by != "" {
				byDate, derr = time.Parse("2006-01-02", by)
			} else {
				derr = fmt.Errorf("missing --by date (YYYY-MM-DD)")
			}
			if terr != nil || derr != nil {
				if dryRunOK(flags) {
					return nil
				}
				if terr != nil {
					return usageErr(terr)
				}
				return usageErr(fmt.Errorf("invalid --by date: %v", derr))
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openDreamingStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			u, ok, err := ensureUser(cmd.Context(), flags, db)
			if err != nil {
				return err
			}
			if !ok {
				return notFoundErr(fmt.Errorf("no user stats cached — run 'dreaming-pp-cli sync' first"))
			}
			current := u.TotalHours()
			remaining := targetHours - current
			days := daysToDate(byDate)
			if paceDays <= 0 {
				paceDays = 30
			}
			series, _ := loadDailyInput(cmd.Context(), db)
			currentPaceMin := hoursFromSeconds(int64(recentAverageSeconds(series, paceDays))) * 60

			reached := remaining <= 0
			var requiredMinPerDay float64
			feasible := true
			if !reached {
				if days <= 0 {
					feasible = false
				} else {
					requiredMinPerDay = (remaining / float64(days)) * 60
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				out := map[string]any{
					"target_hours":         round1(targetHours),
					"current_hours":        round1(current),
					"hours_remaining":      round1(remaining),
					"by_date":              by,
					"days_remaining":       days,
					"required_min_per_day": round1(requiredMinPerDay),
					"current_pace_min_day": round1(currentPaceMin),
					"already_reached":      reached,
					"date_feasible":        feasible,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if reached {
				fmt.Fprintf(cmd.OutOrStdout(), "You're already at %.1f h — target of %.0f h is reached. 🎉\n", current, targetHours)
				return nil
			}
			if !feasible {
				fmt.Fprintf(cmd.OutOrStdout(), "The date %s is not in the future. Pick a later --by date.\n", by)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "To reach %.0f h by %s (%d days):\n", targetHours, by, days)
			fmt.Fprintf(cmd.OutOrStdout(), "  Remaining:       %.1f h\n", remaining)
			fmt.Fprintf(cmd.OutOrStdout(), "  Required pace:   %.0f min/day\n", requiredMinPerDay)
			fmt.Fprintf(cmd.OutOrStdout(), "  Your pace now:   %.0f min/day (last %d days)\n", currentPaceMin, paceDays)
			if currentPaceMin >= requiredMinPerDay {
				fmt.Fprintln(cmd.OutOrStdout(), "  On track at your current pace. ✓")
			} else if currentPaceMin > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Add %.0f min/day to your current pace to make it.\n", requiredMinPerDay-currentPaceMin)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "Target hours (e.g. 600h) or roadmap level (e.g. 5 / L5)")
	cmd.Flags().StringVar(&by, "by", "", "Target date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&paceDays, "pace-days", 30, "Days of recent activity to average for current pace")
	return cmd
}

// parseTargetHours turns "600h", "600", "5", "L5", or "level5" into a target
// hours value (levels resolve via the roadmap ladder).
func parseTargetHours(s string) (float64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("missing --target (e.g. 600h or 5)")
	}
	if strings.HasPrefix(s, "level") {
		s = strings.TrimPrefix(s, "level")
	}
	s = strings.TrimPrefix(s, "l")
	if strings.HasSuffix(s, "h") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "h"), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid --target %q", s)
		}
		return v, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid --target %q: use hours (600h) or a level (5)", s)
	}
	// 1-7 with no 'h' suffix is treated as a roadmap level.
	if v >= 1 && v <= 7 && v == float64(int(v)) {
		for _, r := range roadmapLevels {
			if r.Level == int(v) {
				return r.Hours, nil
			}
		}
	}
	return v, nil
}

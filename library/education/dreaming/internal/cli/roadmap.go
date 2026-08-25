// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local
// roadmap: the full L1-L7 comprehensible-input fluency ladder with advisories
// and personalized calendar ETAs computed from your recent pace. Hand-built
// novel feature. Works offline against the synced store.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type roadmapRung struct {
	Level     int     `json:"level"`
	Hours     float64 `json:"hours_threshold"`
	Note      string  `json:"note"`
	Status    string  `json:"status"` // done | current | future
	ETADate   string  `json:"eta_date,omitempty"`
	ETADays   int     `json:"eta_days,omitempty"`
	HoursToGo float64 `json:"hours_to_go,omitempty"`
}

func newNovelRoadmapCmd(flags *rootFlags) *cobra.Command {
	var paceDays int
	var related bool

	cmd := &cobra.Command{
		Use:   "roadmap",
		Short: "The whole L1-L7 ladder with comprehensible-input advisories and personalized ETAs",
		Long: "Show the full L1-L7 fluency ladder with its comprehensible-input advisories\n" +
			"(speaking optional at 300h, recommended + reading at 600h, native-like at\n" +
			"1500h), your current position, and the projected calendar date you reach\n" +
			"each future level at your recent pace. Use --related to halve thresholds\n" +
			"(rule of thumb when coming from a closely related Romance language).",
		Example: strings.Trim(`
  dreaming-pp-cli roadmap
  dreaming-pp-cli roadmap --pace-days 14 --related
  dreaming-pp-cli roadmap --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
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
			series, _ := loadDailyInput(cmd.Context(), db)
			if paceDays <= 0 {
				paceDays = 30
			}
			paceHrsPerDay := hoursFromSeconds(int64(recentAverageSeconds(series, paceDays)))
			current := u.TotalHours()

			scale := 1.0
			if related {
				scale = 0.5
			}

			var rungs []roadmapRung
			for _, r := range roadmapLevels {
				threshold := r.Hours * scale
				rr := roadmapRung{Level: r.Level, Hours: threshold, Note: r.Note}
				switch {
				case current >= threshold:
					rr.Status = "done"
				default:
					rr.Status = "future"
					rr.HoursToGo = threshold - current
					if paceHrsPerDay > 0 {
						days := int(rr.HoursToGo/paceHrsPerDay + 0.5)
						rr.ETADays = days
						rr.ETADate = time.Now().AddDate(0, 0, days).Format("2006-01-02")
					}
				}
				rungs = append(rungs, rr)
			}
			// Mark the highest done rung as current.
			for i := len(rungs) - 1; i >= 0; i-- {
				if rungs[i].Status == "done" {
					rungs[i].Status = "current"
					break
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				out := map[string]any{
					"total_hours":        round1(current),
					"pace_hours_per_day": round2(paceHrsPerDay),
					"pace_window_days":   paceDays,
					"related_language":   related,
					"current_level":      levelForScaled(current, scale),
					"ladder":             rungs,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cumulative input: %.1f h   |   pace: %.2f h/day (last %d days)\n\n", current, paceHrsPerDay, paceDays)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "\tLEVEL\tHOURS\tETA\tNOTE")
			for _, r := range rungs {
				marker := " "
				switch r.Status {
				case "current":
					marker = "►"
				case "done":
					marker = "✓"
				}
				eta := "—"
				if r.Status == "future" {
					if r.ETADate != "" {
						eta = fmt.Sprintf("%s (%dd)", r.ETADate, r.ETADays)
					} else {
						eta = "set a daily goal"
					}
				}
				fmt.Fprintf(tw, "%s\tL%d\t%.0f\t%s\t%s\n", marker, r.Level, r.Hours, eta, r.Note)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if paceHrsPerDay == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\n(No recent activity in the local store, so ETAs are unavailable. Sync, then log hours to project dates.)")
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&paceDays, "pace-days", 30, "Days of recent activity to average for the pace estimate")
	cmd.Flags().BoolVar(&related, "related", false, "Halve hour thresholds (coming from a closely related Romance language)")
	return cmd
}

func levelForScaled(hours, scale float64) int {
	lvl := 1
	for _, r := range roadmapLevels {
		if hours >= r.Hours*scale {
			lvl = r.Level
		}
	}
	return lvl
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }

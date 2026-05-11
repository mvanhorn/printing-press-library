package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/fathom/internal/store"
	"github.com/spf13/cobra"
)

func newCoverageCmd(flags *rootFlags) *cobra.Command {
	var pattern string
	var weeks int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Recording coverage — track whether a recurring meeting is being recorded reliably",
		Long: `Given a meeting title pattern (e.g. 'Weekly Planning'), show which weeks
had a matching recording and which weeks had gaps. Useful for verifying
that mandatory-record meetings are actually being captured.

Run 'sync --full' first to populate the local store.`,
		Example: strings.Trim(`
  fathom-pp-cli coverage --pattern "Weekly Planning"
  fathom-pp-cli coverage --pattern "standup" --weeks 10
  fathom-pp-cli coverage --pattern "1:1" --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && pattern == "" {
				pattern = args[0]
			}
			if pattern == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fathom-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if weeks <= 0 {
				weeks = 8
			}
			cutoff, _ := parseSince(fmt.Sprintf("%dd", weeks*7))

			meetings, err := loadAllMeetings(cmd.Context(), db)
			if err != nil {
				return err
			}

			// Find meetings matching the pattern
			weekRecorded := map[string][]string{} // week -> meeting titles
			for _, m := range meetings {
				if !containsIgnoreCase(m.meetingTitle(), pattern) {
					continue
				}
				t, err := parseFlexTime(m.CreatedAt)
				if err != nil || (!cutoff.IsZero() && t.Before(cutoff)) {
					continue
				}
				week := isoWeek(t)
				weekRecorded[week] = append(weekRecorded[week], m.meetingTitle())
			}

			// Build all weeks in range
			type weekEntry struct {
				Week     string   `json:"week"`
				Recorded bool     `json:"recorded"`
				Meetings []string `json:"meetings"`
			}

			// PATCH(coverage-gap-weeks): enumerate every week in range, not just recorded ones
			var allWeeks []weekEntry
			now := time.Now()
			for i := weeks - 1; i >= 0; i-- {
				d := now.AddDate(0, 0, -i*7)
				w := isoWeek(d)
				if titles, ok := weekRecorded[w]; ok {
					allWeeks = append(allWeeks, weekEntry{Week: w, Recorded: true, Meetings: titles})
				} else {
					allWeeks = append(allWeeks, weekEntry{Week: w, Recorded: false, Meetings: nil})
				}
			}

			recorded := len(weekRecorded)
			total := weeks
			coveragePct := 0.0
			if total > 0 {
				coveragePct = float64(recorded) / float64(total) * 100
			}

			type coverageResult struct {
				Pattern     string      `json:"pattern"`
				Weeks       int         `json:"weeks_analyzed"`
				Recorded    int         `json:"weeks_recorded"`
				Coverage    float64     `json:"coverage_pct"`
				WeekEntries []weekEntry `json:"week_entries"`
			}

			result := coverageResult{
				Pattern:     pattern,
				Weeks:       total,
				Recorded:    recorded,
				Coverage:    coveragePct,
				WeekEntries: allWeeks,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Coverage for '%s' (last %d weeks)\n\n", pattern, weeks)
			fmt.Fprintf(cmd.OutOrStdout(), "Recorded: %d/%d weeks (%.0f%%)\n\n", recorded, total, coveragePct)
			if len(allWeeks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching meetings found.")
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'fathom-pp-cli sync --full' if the store is empty.")
				return nil
			}
			for _, w := range allWeeks {
				status := "✓"
				label := strings.Join(w.Meetings, "; ")
				if !w.Recorded {
					status = "✗"
					label = "(no recording)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s  %s\n", w.Week, status, label)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&pattern, "pattern", "", "Meeting title pattern to match (case-insensitive substring)")
	cmd.Flags().IntVar(&weeks, "weeks", 8, "Number of weeks to analyze")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

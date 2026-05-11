package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// analyticsCmd provides aggregate production analytics from deliver and feedback logs.
// Cross-references multiple data sources to surface production quality trends.
var analyticsCmd = &cobra.Command{
	Use:   "analytics",
	Short: "Show aggregate production quality analytics across sessions",
	Long: `Cross-references your deliver and feedback logs to produce quality analytics.

Reports:
  - Total sessions logged
  - Average feedback rating
  - Session frequency
  - Rating distribution
  - Most common feedback tags

Use 'obs-pp-cli deliver create' and 'obs-pp-cli feedback add' to build the log.`,
	Example: `  obs-pp-cli analytics
  obs-pp-cli analytics --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := loadDeliveries()
		if err != nil {
			return fmt.Errorf("failed to load delivery log: %w", err)
		}
		feedbackEntries, err := loadFeedback()
		if err != nil {
			return fmt.Errorf("failed to load feedback log: %w", err)
		}

		// Aggregate rating distribution
		ratingDist := make(map[int]int)
		totalRating := 0
		tagCounts := make(map[string]int)
		for _, f := range feedbackEntries {
			ratingDist[f.Rating]++
			totalRating += f.Rating
			for _, tag := range f.Tags {
				tagCounts[tag]++
			}
		}

		avgRating := 0.0
		if len(feedbackEntries) > 0 {
			avgRating = float64(totalRating) / float64(len(feedbackEntries))
		}

		// Completion rate: sessions with status != "pending"
		completed := 0
		for _, s := range sessions {
			if s.Status != "pending" {
				completed++
			}
		}
		completionRate := 0.0
		if len(sessions) > 0 {
			completionRate = float64(completed) / float64(len(sessions)) * 100
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{
				"sessions_total":   len(sessions),
				"sessions_completed": completed,
				"completion_rate_pct": fmt.Sprintf("%.1f%%", completionRate),
				"feedback_count":   len(feedbackEntries),
				"avg_rating":       fmt.Sprintf("%.1f", avgRating),
				"rating_distribution": ratingDist,
				"top_tags":         tagCounts,
			})
		}

		w := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintln(w, "Production Analytics")
		fmt.Fprintln(w, "────────────────────────────────────────")
		fmt.Fprintf(w, "  Sessions logged:\t%d\n", len(sessions))
		fmt.Fprintf(w, "  Feedback entries:\t%d\n", len(feedbackEntries))
		fmt.Fprintf(w, "  Avg rating:\t%.1f / 5.0\n", avgRating)
		fmt.Fprintf(w, "  Completion rate:\t%.1f%%\n", completionRate)

		if len(ratingDist) > 0 {
			fmt.Fprintln(w, "\nRating Distribution:")
			for rating := 5; rating >= 1; rating-- {
				count := ratingDist[rating]
				bar := ""
				for i := 0; i < count; i++ {
					bar += "█"
				}
				fmt.Fprintf(w, "  %d★:\t%s (%d)\n", rating, bar, count)
			}
		}

		if len(tagCounts) > 0 {
			fmt.Fprintln(w, "\nTag Frequency:")
			for tag, count := range tagCounts {
				fmt.Fprintf(w, "  %s:\t%d\n", tag, count)
			}
		}

		if len(sessions) == 0 && len(feedbackEntries) == 0 {
			fmt.Fprintln(w, "\n  No data yet. Start logging with:")
			fmt.Fprintln(w, "    obs-pp-cli deliver create --title \"Episode 1\"")
			fmt.Fprintln(w, "    obs-pp-cli feedback add --rating 5")
		}

		return w.Flush()
	},
}

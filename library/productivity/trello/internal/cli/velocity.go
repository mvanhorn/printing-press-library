// Hand-authored novel command (not generated). Throughput over time from the
// activity log. Counts cards completed per week (moved to a done-like list or
// marked dueComplete / archived) across the last N weeks.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/store"
	"github.com/spf13/cobra"
)

type velocityWeek struct {
	WeekStart string `json:"week_start"`
	Completed int    `json:"completed"`
}

func newNovelVelocityCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var weeks int
	var board string
	cmd := &cobra.Command{
		Use:         "velocity",
		Short:       "Cards completed per week over the last N weeks, per board or member, with trend.",
		Long:        "Use this command to track throughput trends over time. It buckets completion events from the local activity log into weekly counts, which requires historical action snapshots no single Trello API call provides.",
		Example:     "  trello-pp-cli velocity --weeks 8 --agent\n  trello-pp-cli velocity --weeks 4 --board Eng",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if weeks <= 0 {
				weeks = 4
			}
			if dbPath == "" {
				dbPath = defaultDBPath("trello-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'trello-pp-cli sync' first.", err)
			}
			defer db.Close()

			actions, err := loadActions(db)
			if err != nil {
				return err
			}
			boards := nameLookup(db, []string{"boards"})

			now := nowUTC()
			cutoff := now.AddDate(0, 0, -7*weeks)
			buckets := map[string]int{}
			for _, a := range actions {
				if !isCompletionAction(a) {
					continue
				}
				if board != "" && !strings.EqualFold(resolve(boards, a.Data.Board.ID), board) && !strings.EqualFold(a.Data.Board.ID, board) {
					continue
				}
				t, ok := parseTrelloTime(a.Date)
				if !ok || t.Before(cutoff) {
					continue
				}
				wk := weekStart(t).Format("2006-01-02")
				buckets[wk]++
			}

			weeksOut := make([]velocityWeek, 0, len(buckets))
			for k, v := range buckets {
				weeksOut = append(weeksOut, velocityWeek{WeekStart: k, Completed: v})
			}
			sort.Slice(weeksOut, func(i, j int) bool { return weeksOut[i].WeekStart < weeksOut[j].WeekStart })

			total := 0
			for _, w := range weeksOut {
				total += w.Completed
			}
			avg := 0.0
			if len(weeksOut) > 0 {
				avg = float64(total) / float64(len(weeksOut))
			}

			view := map[string]any{
				"weeks_window":    weeks,
				"board":           board,
				"total_completed": total,
				"avg_per_week":    avg,
				"weeks":           weeksOut,
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(weeksOut) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No completion activity found. Sync actions with 'trello-pp-cli sync' first.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Velocity (last %d weeks, avg %.1f/week):\n\n", weeks, avg)
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %s\n", "WEEK", "COMPLETED")
			for _, w := range weeksOut {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %d\n", w.WeekStart, w.Completed)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&weeks, "weeks", 4, "Number of weeks to look back")
	cmd.Flags().StringVar(&board, "board", "", "Filter to a single board by name or id")
	return cmd
}

// isCompletionAction recognizes activity-log events that represent a card being
// completed: marked due-complete, archived (closed), or moved into a list whose
// name reads as done/complete/shipped.
func isCompletionAction(a trelloAction) bool {
	switch a.Type {
	case "updateCard":
		after := strings.ToLower(a.Data.ListAfter.Name)
		if after != "" && (strings.Contains(after, "done") || strings.Contains(after, "complete") || strings.Contains(after, "shipped") || strings.Contains(after, "closed")) {
			return true
		}
	}
	return false
}

func weekStart(t time.Time) time.Time {
	t = t.UTC()
	// ISO week: roll back to Monday.
	offset := (int(t.Weekday()) + 6) % 7
	d := t.AddDate(0, 0, -offset)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

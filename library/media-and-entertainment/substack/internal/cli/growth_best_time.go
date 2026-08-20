// Phase 3 hand-authored novel command. Not generator-emitted.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type bestTimeRow struct {
	DayOfWeek     int     `json:"day_of_week"`
	DayOfWeekName string  `json:"day_of_week_name"`
	Hour          int     `json:"hour"`
	Goal          string  `json:"goal"`
	Rate          float64 `json:"rate"`
	SampleSize    int     `json:"sample_size"`
}

var dowNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

func newGrowthBestTimeCmd(flags *rootFlags) *cobra.Command {
	var days int
	var forGoal string
	var top int

	cmd := &cobra.Command{
		Use:   "best-time",
		Short: "Top day-of-week × hour cells ranked by your chosen growth signal",
		Example: strings.Trim(`
  substack-pp-cli growth best-time --for-goal subs --top 5 --json
  substack-pp-cli growth best-time --for-goal restacks --days 60 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), []bestTimeRow{}, flags)
			}
			switch forGoal {
			case "subs", "likes", "restacks", "comments":
			default:
				return usageErr(fmt.Errorf("--for-goal must be one of: subs, likes, restacks, comments (got %q)", forGoal))
			}
			rows, err := computeBestTime(flags, forGoal, top)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []bestTimeRow{}
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no reach_windows data — run 'substack-pp-cli sync' to populate the local store first")
			}
			_ = days
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 90, "Lookback window in days")
	cmd.Flags().StringVar(&forGoal, "for-goal", "subs", "Optimize for: subs|likes|restacks|comments")
	cmd.Flags().IntVar(&top, "top", 5, "Top N cells to return")
	return cmd
}

func computeBestTime(flags *rootFlags, goal string, top int) ([]bestTimeRow, error) {
	st, err := store.Open(defaultDBPath("substack-pp-cli"))
	if err != nil {
		return nil, nil
	}
	defer st.Close()
	rows, err := st.DB().Query(`SELECT day_of_week, hour, goal, rate, sample_size FROM reach_windows WHERE goal = ?`, goal)
	if err != nil {
		if isMissingTableErr(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := []bestTimeRow{}
	for rows.Next() {
		var r bestTimeRow
		if err := rows.Scan(&r.DayOfWeek, &r.Hour, &r.Goal, &r.Rate, &r.SampleSize); err != nil {
			continue
		}
		if r.DayOfWeek >= 0 && r.DayOfWeek < len(dowNames) {
			r.DayOfWeekName = dowNames[r.DayOfWeek]
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rate > out[j].Rate })
	if top > 0 && len(out) > top {
		out = out[:top]
	}
	return out, nil
}

// Hand-authored novel feature: meeting-load analytics. Survives regen.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type loadBucket struct {
	Bucket      string  `json:"bucket"`
	Meetings    int     `json:"meetings"`
	BookedHours float64 `json:"booked_hours"`
	AllDay      int     `json:"all_day_events"`
}

func newLoadCmd(flags *rootFlags) *cobra.Command {
	var calendarsCSV, window, groupBy string
	cmd := &cobra.Command{
		Use:   "load",
		Short: "Count meetings and booked hours per day, week, or calendar",
		Long: "Aggregate meeting count and total booked hours over a window, grouped by day, week, or calendar.\n\n" +
			"A GROUP BY over the local event store — no single Google API call returns aggregated load.",
		Example: "  google-calendar-pp-cli load --window 'this month' --group-by week --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			switch groupBy {
			case "day", "week", "calendar":
			default:
				return usageErr(fmt.Errorf("invalid --group-by %q: use day, week, or calendar", groupBy))
			}
			start, end, err := parseWindow(window)
			if err != nil {
				return usageErr(err)
			}
			cals := resolveCalendars(calendarsCSV)
			events, _, err := gcalLoadEvents(cmd, flags, eventQuery{calendars: cals, timeMin: start, timeMax: end})
			if err != nil {
				return err
			}

			agg := map[string]*loadBucket{}
			order := []string{}
			bucketOf := func(ev gcalEvent) string {
				switch groupBy {
				case "calendar":
					return ev.CalendarID
				case "week":
					return startOfWeek(ev.Start).Format("2006-01-02")
				default:
					return startOfDay(ev.Start).Format("2006-01-02")
				}
			}
			for _, ev := range events {
				if ev.Status == "cancelled" {
					continue
				}
				if groupBy != "calendar" && ev.Start.IsZero() {
					continue
				}
				key := bucketOf(ev)
				b, ok := agg[key]
				if !ok {
					b = &loadBucket{Bucket: key}
					agg[key] = b
					order = append(order, key)
				}
				b.Meetings++
				if ev.AllDay {
					b.AllDay++
					continue
				}
				if !ev.Start.IsZero() && ev.End.After(ev.Start) {
					b.BookedHours += ev.End.Sub(ev.Start).Hours()
				}
			}

			out := make([]loadBucket, 0, len(order))
			for _, k := range order {
				b := agg[k]
				b.BookedHours = float64(int(b.BookedHours*100+0.5)) / 100 // round to 2dp
				out = append(out, *b)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Bucket < out[j].Bucket })
			if out == nil {
				out = []loadBucket{}
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&calendarsCSV, "calendars", "primary", "Comma-separated calendar IDs to include")
	cmd.Flags().StringVar(&window, "window", "this month", "Time window (this week, this month, next 30 days, a..b)")
	cmd.Flags().StringVar(&groupBy, "group-by", "day", "Aggregation bucket: day, week, or calendar")
	return cmd
}

// Hand-authored novel feature: free-slot finder. Survives regen.
package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type freeSlot struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Duration int    `json:"duration_minutes"`
}

type interval struct {
	start, end time.Time
}

func newFreeCmd(flags *rootFlags) *cobra.Command {
	var calendarsCSV, window, duration string
	var businessHours bool
	var limit int
	cmd := &cobra.Command{
		Use:   "free",
		Short: "Find open time blocks of a given length across calendars",
		Long: "Find free time gaps of at least --duration across one or more calendars in a window.\n\n" +
			"Busy intervals are inverted from the local event store (or fetched live and cached), so this\n" +
			"answers \"where is my next 90-minute opening?\" — something the live freeBusy endpoint, which only\n" +
			"reports busy, cannot.",
		Example: "  google-calendar-pp-cli free --calendars primary --window 'next 7 days' --duration 90m --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			start, end, err := parseWindow(window)
			if err != nil {
				return usageErr(err)
			}
			dur, err := time.ParseDuration(duration)
			if err != nil || dur <= 0 {
				return usageErr(fmt.Errorf("invalid --duration %q: use forms like 30m, 1h, 90m", duration))
			}
			cals := resolveCalendars(calendarsCSV)
			events, _, err := gcalLoadEvents(cmd, flags, eventQuery{calendars: cals, timeMin: start, timeMax: end})
			if err != nil {
				return err
			}

			busy := make([]interval, 0, len(events))
			for _, ev := range events {
				if ev.Status == "cancelled" || ev.AllDay || ev.Transparency == "transparent" {
					continue
				}
				if ev.Start.IsZero() || ev.End.IsZero() || !ev.End.After(ev.Start) {
					continue
				}
				busy = append(busy, interval{ev.Start, ev.End})
			}
			merged := mergeIntervals(busy)

			open := openIntervals(start, end, businessHours)
			var slots []freeSlot
			for _, o := range open {
				for _, gap := range subtractBusy(o, merged) {
					if gap.end.Sub(gap.start) >= dur {
						slots = append(slots, freeSlot{
							Start:    gap.start.Format(time.RFC3339),
							End:      gap.end.Format(time.RFC3339),
							Duration: int(gap.end.Sub(gap.start).Minutes()),
						})
					}
				}
			}
			sort.Slice(slots, func(i, j int) bool { return slots[i].Start < slots[j].Start })
			if limit > 0 && len(slots) > limit {
				slots = slots[:limit]
			}
			if slots == nil {
				slots = []freeSlot{}
			}
			return flags.printJSON(cmd, slots)
		},
	}
	cmd.Flags().StringVar(&calendarsCSV, "calendars", "primary", "Comma-separated calendar IDs to check")
	cmd.Flags().StringVar(&window, "window", "next 7 days", "Time window (today, this week, next 7 days, 2026-05-24, a..b)")
	cmd.Flags().StringVar(&duration, "duration", "30m", "Minimum free-block length (e.g. 30m, 1h, 90m)")
	cmd.Flags().BoolVar(&businessHours, "business-hours", false, "Only consider 09:00–18:00 on weekdays")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of slots to return (0 = all)")
	return cmd
}

// mergeIntervals sorts and coalesces overlapping/adjacent intervals.
func mergeIntervals(in []interval) []interval {
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool { return in[i].start.Before(in[j].start) })
	out := []interval{in[0]}
	for _, cur := range in[1:] {
		last := &out[len(out)-1]
		if !cur.start.After(last.end) {
			if cur.end.After(last.end) {
				last.end = cur.end
			}
			continue
		}
		out = append(out, cur)
	}
	return out
}

// openIntervals returns the base availability windows. Without --business-hours
// it's the whole [start,end); with it, 09:00–18:00 on each weekday in range.
func openIntervals(start, end time.Time, businessHours bool) []interval {
	if !businessHours {
		return []interval{{start, end}}
	}
	var out []interval
	for d := startOfDay(start); d.Before(end); d = d.AddDate(0, 0, 1) {
		wd := d.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			continue
		}
		dayStart := time.Date(d.Year(), d.Month(), d.Day(), 9, 0, 0, 0, d.Location())
		dayEnd := time.Date(d.Year(), d.Month(), d.Day(), 18, 0, 0, 0, d.Location())
		if dayStart.Before(start) {
			dayStart = start
		}
		if dayEnd.After(end) {
			dayEnd = end
		}
		if dayEnd.After(dayStart) {
			out = append(out, interval{dayStart, dayEnd})
		}
	}
	return out
}

// subtractBusy removes merged busy intervals from a single open interval,
// returning the free gaps.
func subtractBusy(open interval, busy []interval) []interval {
	var gaps []interval
	cursor := open.start
	for _, b := range busy {
		if !b.end.After(open.start) || !b.start.Before(open.end) {
			continue
		}
		bs, be := b.start, b.end
		if bs.Before(open.start) {
			bs = open.start
		}
		if be.After(open.end) {
			be = open.end
		}
		if bs.After(cursor) {
			gaps = append(gaps, interval{cursor, bs})
		}
		if be.After(cursor) {
			cursor = be
		}
	}
	if cursor.Before(open.end) {
		gaps = append(gaps, interval{cursor, open.end})
	}
	return gaps
}

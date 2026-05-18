// Hand-built (Phase 3): free-time finder. Walks merged busy intervals from the
// local store, subtracts from a working-hours window, returns gaps >= duration.

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-calendar/internal/store"

	"github.com/spf13/cobra"
)

func newFreetimeCmd(flags *rootFlags) *cobra.Command {
	var duration string
	var within string
	var nextSpec string
	var excludeOOF bool
	var excludeTentative bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "freetime",
		Short: "Find N-minute gaps in working hours over a date window using locally synced calendars",
		Long: `Computes free-time gaps by merging busy intervals from your synced events,
subtracting them from a working-hours window across the next K days, and
returning gaps at least --duration long. The --within flag accepts ranges
like "Mon-Fri 9-17" or "Sun-Sat 8-20"; pass "any" to ignore working hours.`,
		Example: strings.Trim(`
  outlook-calendar-pp-cli freetime --duration 60m --within "Mon-Fri 9-17" --next 7d --json
  outlook-calendar-pp-cli freetime --duration 30m --within any --next 2d --exclude-oof --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dur, err := time.ParseDuration(duration)
			if err != nil || dur <= 0 {
				return usageErr(fmt.Errorf("--duration must be a positive Go duration like 30m or 1h: %w", err))
			}
			window, err := parseRelativeDays(nextSpec)
			if err != nil {
				return usageErr(fmt.Errorf("--next %q: %w", nextSpec, err))
			}
			wh, err := parseWorkingHours(within)
			if err != nil {
				return usageErr(fmt.Errorf("--within %q: %w", within, err))
			}

			now := time.Now()
			start := now
			end := now.Add(time.Duration(window) * 24 * time.Hour)

			if dbPath == "" {
				dbPath = defaultDBPath("outlook-calendar-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			events, err := loadEvents(cmd.Context(), db.DB(), start, end)
			if err != nil {
				return apiErr(err)
			}

			var busy []freetimeInterval
			for _, ev := range events {
				if ev.IsCancelled {
					continue
				}
				switch strings.ToLower(ev.ShowAs) {
				case "free":
					continue
				case "tentative":
					if excludeTentative {
						continue
					}
				case "oof":
					if excludeOOF {
						continue
					}
				}
				s, err := parseGraphTime(ev.Start)
				if err != nil {
					continue
				}
				e, err := parseGraphTime(ev.End)
				if err != nil {
					continue
				}
				busy = append(busy, freetimeInterval{start: s, end: e})
			}
			busy = mergeIntervals(busy)

			type gap struct {
				Start    string `json:"start"`
				End      string `json:"end"`
				Minutes  int    `json:"minutes"`
				Duration string `json:"duration"`
			}
			gaps := []gap{}

			emit := func(s, e time.Time) {
				if e.Sub(s) < dur {
					return
				}
				gaps = append(gaps, gap{
					Start:    s.Format(time.RFC3339),
					End:      e.Format(time.RFC3339),
					Minutes:  int(e.Sub(s).Minutes()),
					Duration: e.Sub(s).String(),
				})
			}

			// Iterate day by day so working hours are honored per-day.
			for day := 0; day <= window; day++ {
				cursor := start.AddDate(0, 0, day)
				weekday := cursor.Weekday()
				dayStart, dayEnd := wh.Span(cursor, weekday)
				if dayStart.IsZero() || dayEnd.IsZero() {
					continue
				}
				if dayEnd.Before(start) {
					continue
				}
				if dayStart.After(end) {
					break
				}
				if dayStart.Before(start) {
					dayStart = start
				}
				if dayEnd.After(end) {
					dayEnd = end
				}

				cur := dayStart
				for _, b := range busy {
					if b.end.Before(cur) {
						continue
					}
					if b.start.After(dayEnd) {
						break
					}
					if b.start.After(cur) {
						emit(cur, minTime(b.start, dayEnd))
					}
					if b.end.After(cur) {
						cur = b.end
					}
					if cur.After(dayEnd) {
						break
					}
				}
				if cur.Before(dayEnd) {
					emit(cur, dayEnd)
				}
			}

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				return printJSONFiltered(w, gaps, flags)
			}
			if len(gaps) == 0 {
				fmt.Fprintln(w, "No free slots in the requested window.")
				return nil
			}
			fmt.Fprintf(w, "%d open slot(s) >= %s:\n", len(gaps), dur)
			for _, g := range gaps {
				fmt.Fprintf(w, "  %s → %s   (%d min)\n", g.Start, g.End, g.Minutes)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&duration, "duration", "30m", "Minimum gap duration (Go duration: 30m, 1h, 90m)")
	cmd.Flags().StringVar(&within, "within", "Mon-Fri 9-17", "Working-hours spec (e.g. 'Mon-Fri 9-17', 'Sun-Sat 8-20', or 'any')")
	cmd.Flags().StringVar(&nextSpec, "next", "7d", "Look ahead N days (e.g. 7d, 14d)")
	cmd.Flags().BoolVar(&excludeOOF, "exclude-oof", false, "Skip events marked Out Of Office (count those as free)")
	cmd.Flags().BoolVar(&excludeTentative, "exclude-tentative", false, "Skip events marked Tentative (count those as free)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type workingHours struct {
	any  bool
	days map[time.Weekday]bool
	from int // hour 0..24
	to   int // hour 0..24
}

func (w workingHours) Span(day time.Time, weekday time.Weekday) (time.Time, time.Time) {
	if w.any {
		s := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
		return s, s.Add(24 * time.Hour)
	}
	if !w.days[weekday] {
		return time.Time{}, time.Time{}
	}
	s := time.Date(day.Year(), day.Month(), day.Day(), w.from, 0, 0, 0, day.Location())
	e := time.Date(day.Year(), day.Month(), day.Day(), w.to, 0, 0, 0, day.Location())
	return s, e
}

var weekdayShort = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
	"wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

func parseWorkingHours(spec string) (workingHours, error) {
	spec = strings.TrimSpace(spec)
	if strings.EqualFold(spec, "any") || spec == "" {
		return workingHours{any: true}, nil
	}
	parts := strings.Fields(spec)
	if len(parts) != 2 {
		return workingHours{}, fmt.Errorf("expected 'Day-Day H-H' (e.g. 'Mon-Fri 9-17')")
	}
	dayPart := strings.ToLower(parts[0])
	hourPart := parts[1]
	dayRange := strings.Split(dayPart, "-")
	if len(dayRange) != 2 {
		return workingHours{}, fmt.Errorf("day range must be 'Start-End' (e.g. 'Mon-Fri')")
	}
	from, ok1 := weekdayShort[dayRange[0][:3]]
	to, ok2 := weekdayShort[dayRange[1][:3]]
	if !ok1 || !ok2 {
		return workingHours{}, fmt.Errorf("unknown weekday in %q", dayPart)
	}
	hourRange := strings.Split(hourPart, "-")
	if len(hourRange) != 2 {
		return workingHours{}, fmt.Errorf("hour range must be 'Start-End' (e.g. '9-17')")
	}
	hFrom, err := strconv.Atoi(hourRange[0])
	if err != nil {
		return workingHours{}, fmt.Errorf("hour-from %q is not an integer", hourRange[0])
	}
	hTo, err := strconv.Atoi(hourRange[1])
	if err != nil {
		return workingHours{}, fmt.Errorf("hour-to %q is not an integer", hourRange[1])
	}
	if hFrom < 0 || hFrom > 24 || hTo < 0 || hTo > 24 || hFrom >= hTo {
		return workingHours{}, fmt.Errorf("hours must satisfy 0 <= from < to <= 24")
	}

	wh := workingHours{from: hFrom, to: hTo, days: map[time.Weekday]bool{}}
	d := from
	for {
		wh.days[d] = true
		if d == to {
			break
		}
		d = (d + 1) % 7
	}
	return wh, nil
}

func parseRelativeDays(s string) (int, error) {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, "d") {
		return 0, fmt.Errorf("expected '<N>d' (e.g. 7d)")
	}
	n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
	if err != nil || n <= 0 || n > 365 {
		return 0, fmt.Errorf("days must be in 1..365")
	}
	return n, nil
}

type freetimeInterval struct {
	start, end time.Time
}

func mergeIntervals(in []freetimeInterval) []freetimeInterval {
	if len(in) == 0 {
		return in
	}
	sort.Slice(in, func(i, j int) bool { return in[i].start.Before(in[j].start) })
	out := []freetimeInterval{in[0]}
	for _, iv := range in[1:] {
		last := &out[len(out)-1]
		if !iv.start.After(last.end) {
			if iv.end.After(last.end) {
				last.end = iv.end
			}
			continue
		}
		out = append(out, iv)
	}
	return out
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

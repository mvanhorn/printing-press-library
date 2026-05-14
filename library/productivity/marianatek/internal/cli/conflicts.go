// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/marianatek/internal/store"
	"github.com/spf13/cobra"
)

func newConflictsCmd(flags *rootFlags) *cobra.Command {
	var icsPath string
	var buffer time.Duration
	var dbPath string

	cmd := &cobra.Command{
		Use:   "conflicts [date]",
		Short: "Flag overlapping reservations and back-to-back pairs under --buffer for a date",
		Long: `conflicts reads your synced reservations for the given date and detects:
  - Overlapping intervals (one class starts before another ends)
  - Pairs separated by less than --buffer (insufficient travel/recovery time)

Optionally accepts an exported ICS calendar via --ics so personal events
participate in the conflict detection too. Date format: YYYY-MM-DD.

Note: in v0.1 conflicts queries the single tenant the CLI is configured for;
multi-tenant aggregation is planned for v0.2.`,
		Example: `  # Conflicts for May 15 with a 30-min buffer
  marianatek-pp-cli conflicts 2026-05-15 --buffer 30m

  # Include personal calendar events
  marianatek-pp-cli conflicts 2026-05-15 --ics ~/Downloads/calendar.ics --json`,
		Annotations: map[string]string{
			"pp:novel":      "conflicts",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			date, err := time.Parse("2006-01-02", args[0])
			if err != nil {
				return usageErr(fmt.Errorf("date must be YYYY-MM-DD: %w", err))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("marianatek-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			events := collectReservationEvents(db, date)
			if icsPath != "" {
				ics, err := parseICSEvents(icsPath, date)
				if err != nil {
					return fmt.Errorf("parsing %s: %w", icsPath, err)
				}
				events = append(events, ics...)
			}
			sort.Slice(events, func(i, j int) bool { return events[i].Start.Before(events[j].Start) })

			conflicts := detectConflicts(events, buffer)
			return printJSONFiltered(cmd.OutOrStdout(), conflicts, flags)
		},
	}
	cmd.Flags().StringVar(&icsPath, "ics", "", "path to an exported ICS calendar (optional)")
	cmd.Flags().DurationVar(&buffer, "buffer", 30*time.Minute, "minimum gap between events (back-to-back tolerance)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default: ~/.local/share/marianatek-pp-cli/data.db)")
	return cmd
}

type conflictEvent struct {
	Source string    `json:"source"`
	Title  string    `json:"title"`
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
}

type conflictPair struct {
	Kind string        `json:"kind"`
	A    conflictEvent `json:"a"`
	B    conflictEvent `json:"b"`
	Gap  string        `json:"gap,omitempty"`
}

func collectReservationEvents(db *store.Store, date time.Time) []conflictEvent {
	out := []conflictEvent{}
	rows, err := db.List("me_reservations", 1000)
	if err != nil {
		return out
	}
	classes, _ := db.List("classes", 5000)
	classByID := indexClassesByID(classes)

	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	for _, raw := range rows {
		var rec map[string]any
		if err := json.Unmarshal(raw, &rec); err != nil {
			continue
		}
		attrs := pickAttrs(rec)
		classID := stringAttr(attrs, "class_session_id", "class_id")
		var cattrs map[string]any
		if classID != "" {
			cattrs = classByID[classID]
		}
		if cattrs == nil {
			cattrs = attrs
		}
		start := parseStart(cattrs)
		if start.IsZero() || start.Before(dayStart) || start.After(dayEnd) {
			continue
		}
		duration := 60 // default 60 min
		if mins := intAttr(cattrs, "duration", "duration_minutes"); mins > 0 {
			duration = mins
		}
		out = append(out, conflictEvent{
			Source: "marianatek",
			Title: fmt.Sprintf("%s @ %s",
				stringAttr(cattrs, "name", "class_type_name"),
				stringAttr(cattrs, "location_name", "location")),
			Start: start,
			End:   start.Add(time.Duration(duration) * time.Minute),
		})
	}
	return out
}

func parseICSEvents(path string, date time.Time) ([]conflictEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := []conflictEvent{}
	scanner := bufio.NewScanner(f)
	var ev conflictEvent
	var inEvent bool
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		switch {
		case line == "BEGIN:VEVENT":
			inEvent = true
			ev = conflictEvent{Source: "ics"}
		case line == "END:VEVENT":
			if inEvent && !ev.Start.IsZero() && sameCalendarDate(ev.Start, date) {
				if ev.End.IsZero() {
					ev.End = ev.Start.Add(time.Hour)
				}
				out = append(out, ev)
			}
			inEvent = false
		case inEvent && strings.HasPrefix(line, "SUMMARY:"):
			ev.Title = strings.TrimPrefix(line, "SUMMARY:")
		case inEvent && strings.HasPrefix(line, "DTSTART"):
			ev.Start = parseICSDate(line)
		case inEvent && strings.HasPrefix(line, "DTEND"):
			ev.End = parseICSDate(line)
		}
	}
	return out, scanner.Err()
}

func parseICSDate(line string) time.Time {
	colon := strings.Index(line, ":")
	if colon == -1 {
		return time.Time{}
	}
	prop := line[:colon]
	val := line[colon+1:]
	if strings.HasSuffix(val, "Z") {
		if t, err := time.Parse("20060102T150405Z", val); err == nil {
			return t
		}
	}
	loc := time.UTC
	if tzid := icsParam(prop, "TZID"); tzid != "" {
		loaded, err := time.LoadLocation(tzid)
		if err == nil {
			loc = loaded
		}
	}
	for _, layout := range []string{"20060102T150405", "20060102"} {
		if t, err := time.ParseInLocation(layout, val, loc); err == nil {
			return t
		}
	}
	return time.Time{}
}

func icsParam(prop, name string) string {
	for _, part := range strings.Split(prop, ";")[1:] {
		key, value, ok := strings.Cut(part, "=")
		if !ok || !strings.EqualFold(key, name) {
			continue
		}
		return strings.Trim(value, `"`)
	}
	return ""
}

func sameCalendarDate(t, date time.Time) bool {
	local := t.In(t.Location())
	y, m, d := local.Date()
	return y == date.Year() && m == date.Month() && d == date.Day()
}

func detectConflicts(events []conflictEvent, buffer time.Duration) []conflictPair {
	out := []conflictPair{}
	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			a, b := events[i], events[j]
			if b.Start.Before(a.End) {
				out = append(out, conflictPair{Kind: "overlap", A: a, B: b})
				continue
			}
			gap := b.Start.Sub(a.End)
			if gap < buffer {
				out = append(out, conflictPair{Kind: "tight_gap", A: a, B: b, Gap: gap.String()})
			}
		}
	}
	return out
}

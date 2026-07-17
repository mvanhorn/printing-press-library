// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0.
// Novel feature: season calendar view with optional ICS export.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/motogp/internal/cliutil"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelCalendarCmd(flags *rootFlags) *cobra.Command {
	var flagIcs string
	var includeTests bool
	cmd := &cobra.Command{
		Use:   "calendar <year>",
		Short: "Show a season race calendar, or export it to an ICS file.",
		Long: "Lists a season's rounds (circuit, country, dates) from the broadcast calendar.\n" +
			"Pass --ics <file> to write an importable ICS calendar for any calendar app.",
		Example: strings.Trim(`
  motogp-pp-cli calendar 2026 --agent
  motogp-pp-cli calendar 2026 --ics motogp-2026.ics`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				return usageErr(fmt.Errorf("need <year>, e.g. calendar 2026"))
			}
			year, err := parseYearArg(args[0])
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			raw, err := novelFetch(ctx, c, flags, "auto", "broadcast-events", true, "/events", map[string]string{"seasonYear": fmt.Sprintf("%d", year)})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var bevents []mgpBroadcastEvent
			if err := json.Unmarshal(raw, &bevents); err != nil {
				return classifyAPIError(err, flags)
			}
			// Keep only actual Grand Prix rounds (kind == GP). MEDIA events
			// (team/livery presentations) and, unless --tests, TEST events are dropped.
			var events []mgpEvent
			for _, be := range bevents {
				if be.isGP() {
					events = append(events, be.toEvent())
					continue
				}
				if includeTests && be.isTest() {
					events = append(events, be.toEvent())
				}
			}
			sortEventsByStart(events)

			type outRow struct {
				Round     int    `json:"round"`
				Event     string `json:"event"`
				Circuit   string `json:"circuit"`
				Country   string `json:"country"`
				DateStart string `json:"date_start"`
				DateEnd   string `json:"date_end"`
			}
			out := struct {
				Year   int      `json:"year"`
				Rounds int      `json:"rounds"`
				Events []outRow `json:"events"`
			}{Year: year, Rounds: len(events)}
			for i, ev := range events {
				out.Events = append(out.Events, outRow{
					Round:     i + 1,
					Event:     ev.label(),
					Circuit:   ev.Circuit.Name,
					Country:   ev.Country.Name,
					DateStart: ev.DateStart,
					DateEnd:   ev.DateEnd,
				})
			}

			if flagIcs != "" {
				ics := buildICS(year, events)
				if cliutil.IsVerifyEnv() {
					fmt.Fprintf(cmd.OutOrStdout(), "would write %d events to %s\n", len(events), flagIcs)
					return nil
				}
				if err := os.WriteFile(flagIcs, []byte(ics), 0o644); err != nil {
					return apiErr(fmt.Errorf("writing ICS file: %w", err))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %d events to %s\n", len(events), flagIcs)
				return nil
			}

			if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d calendar: %d rounds\n", out.Year, out.Rounds)
			tableRows := make([][]string, 0, len(out.Events))
			for _, r := range out.Events {
				tableRows = append(tableRows, []string{
					fmt.Sprintf("%d", r.Round), r.Event, r.Circuit, r.Country, r.DateStart,
				})
			}
			return flags.printTable(cmd, []string{"RND", "EVENT", "CIRCUIT", "COUNTRY", "STARTS"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&flagIcs, "ics", "", "Write the calendar to this ICS file instead of printing")
	cmd.Flags().BoolVar(&includeTests, "tests", false, "Include pre-season/test events")
	return cmd
}

// mgpBroadcastEvent models the /events (broadcast) response, whose shape
// differs from /results/events: country is an ISO string and circuit carries
// the full country name.
type mgpBroadcastEvent struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Shortname string `json:"shortname"`
	Kind      string `json:"kind"`
	Type      string `json:"type"`
	DateStart string `json:"date_start"`
	DateEnd   string `json:"date_end"`
	Circuit   struct {
		Name    string `json:"name"`
		Country string `json:"country"`
	} `json:"circuit"`
}

func (b mgpBroadcastEvent) isGP() bool {
	return strings.EqualFold(b.Kind, "GP")
}

func (b mgpBroadcastEvent) isTest() bool {
	return strings.EqualFold(b.Kind, "test")
}

func (b mgpBroadcastEvent) toEvent() mgpEvent {
	return mgpEvent{
		ID:        b.ID,
		Name:      b.Name,
		ShortName: b.Shortname,
		DateStart: b.DateStart,
		DateEnd:   b.DateEnd,
		Circuit:   mgpNamed{Name: b.Circuit.Name},
		Country:   mgpNamed{Name: b.Circuit.Country},
	}
}

// buildICS renders a minimal all-day VEVENT per round.
func buildICS(year int, events []mgpEvent) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//motogp-pp-cli//EN\r\n")
	b.WriteString(fmt.Sprintf("X-WR-CALNAME:MotoGP %d\r\n", year))
	for i, ev := range events {
		start := icsDate(ev.DateStart)
		end := icsDate(ev.DateEnd)
		if start == "" {
			continue
		}
		if end == "" {
			end = start
		}
		summary := ev.label()
		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(fmt.Sprintf("UID:motogp-%d-%d@motogp-pp-cli\r\n", year, i+1))
		b.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", icsEscape(summary)))
		if ev.Circuit.Name != "" {
			b.WriteString(fmt.Sprintf("LOCATION:%s\r\n", icsEscape(ev.Circuit.Name)))
		}
		b.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", start))
		b.WriteString(fmt.Sprintf("DTEND;VALUE=DATE:%s\r\n", icsDatePlusOne(end)))
		b.WriteString("END:VEVENT\r\n")
	}
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

// icsDate extracts YYYYMMDD from an ISO-ish date string.
func icsDate(s string) string {
	if len(s) < 10 {
		return ""
	}
	d := s[:10] // YYYY-MM-DD
	return strings.ReplaceAll(d, "-", "")
}

// icsDatePlusOne bumps an all-day DTEND by one day. An all-day iCalendar DTEND
// is exclusive, so a race whose last day is D must encode DTEND as D+1;
// returning D unchanged yields DTSTART == DTEND, which strict clients import as
// a zero-duration event or drop entirely. Falls back to the input only when it
// cannot be parsed as YYYYMMDD.
func icsDatePlusOne(yyyymmdd string) string {
	t, err := time.Parse("20060102", yyyymmdd)
	if err != nil {
		return yyyymmdd
	}
	return t.AddDate(0, 0, 1).Format("20060102")
}

func icsEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

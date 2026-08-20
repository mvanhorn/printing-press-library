// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"hash/fnv"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelIcalCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "ical",
		Short: "Emit the merged schedule as an RFC-5545 .ics file for phone-calendar subscription.",
		Long: "Write the next N days of merged events as an RFC-5545 iCalendar (.ics) document to stdout. " +
			"Redirect it to a file you can import or host:\n\n  sprocket-pp-cli ical --days 60 > jfc.ics",
		Example:     "  sprocket-pp-cli ical --days 60 > jfc.ics",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			if days < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be at least 1"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now()
			events, err := fetchCalendar(ctx, c, now, now.AddDate(0, 0, days))
			if err != nil {
				return err
			}
			ics := buildICS(events, now)
			// Default output is the raw .ics document (so `ical > jfc.ics` and a
			// piped `ical` both produce a valid calendar file). Under explicit
			// --json/--agent, wrap it so machine consumers get parseable output.
			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, map[string]any{"format": "ics", "content": ics})
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), ics)
			return err
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "number of days ahead to include")
	return cmd
}

// buildICS serializes events to an RFC-5545 VCALENDAR document. Times are
// emitted as floating local wall-clock (the upstream events carry no reliable
// zone), which subscribing calendars interpret in the viewer's timezone. Pure
// and unit-tested.
func buildICS(events []calEvent, now time.Time) string {
	var b strings.Builder
	// Fold each content line per RFC 5545 §3.1 before terminating with CRLF.
	crlf := func(s string) { b.WriteString(foldICSLine(s)); b.WriteString("\r\n") }
	crlf("BEGIN:VCALENDAR")
	crlf("VERSION:2.0")
	crlf("PRODID:-//sprocket-pp-cli//Sprocket Sports CLI//EN")
	crlf("CALSCALE:GREGORIAN")
	stamp := now.UTC().Format("20060102T150405Z")
	for _, e := range events {
		if !e.HasStart {
			continue
		}
		crlf("BEGIN:VEVENT")
		crlf("UID:" + icalUID(e))
		crlf("DTSTAMP:" + stamp)
		crlf("DTSTART:" + e.Start.Format("20060102T150405"))
		end := e.End
		if !end.After(e.Start) {
			end = e.Start.Add(defaultEventDuration)
		}
		crlf("DTEND:" + end.Format("20060102T150405"))
		crlf("SUMMARY:" + icalEscape(icalSummary(e)))
		if e.Location != "" {
			crlf("LOCATION:" + icalEscape(e.Location))
		}
		if e.Cancelled {
			crlf("STATUS:CANCELLED")
		} else {
			crlf("STATUS:CONFIRMED")
		}
		crlf("END:VEVENT")
	}
	crlf("END:VCALENDAR")
	return b.String()
}

func icalUID(e calEvent) string {
	if e.ID != 0 {
		return fmt.Sprintf("sprocket-%d@sprocketsports.com", e.ID)
	}
	// Zero-ID events: a start timestamp alone is not unique — two of a family's
	// children can have distinct events at the same time, and identical UIDs
	// make calendar apps silently drop one (defeating the multi-child merge).
	// Disambiguate with a stable content hash of the distinguishing fields so
	// distinct events get distinct UIDs while re-imports of the same event stay
	// stable.
	h := fnv.New32a()
	_, _ = io.WriteString(h, e.StartRaw+"\x00"+e.EndRaw+"\x00"+e.Title+"\x00"+e.Opponent+"\x00"+strconv.Itoa(e.LocationID))
	return fmt.Sprintf("sprocket-%s-%08x@sprocketsports.com", e.Start.Format("20060102T150405"), h.Sum32())
}

func icalSummary(e calEvent) string {
	s := e.Title
	if e.Opponent != "" {
		s = fmt.Sprintf("%s vs %s", s, e.Opponent)
	}
	return s
}

// foldICSLine folds a content line to the RFC 5545 §3.1 limit of 75 octets:
// the first segment is at most 75 octets, and each continuation line begins
// with a single space (which counts toward its 75-octet budget, so 74 octets
// of payload). Folding is octet-based per the spec; a multi-byte rune may be
// split across a fold and is reassembled by the unfolding parser before
// charset decoding.
func foldICSLine(s string) string {
	const limit = 75
	if len(s) <= limit {
		return s
	}
	var b strings.Builder
	b.WriteString(s[:limit])
	for rest := s[limit:]; len(rest) > 0; {
		n := 74
		if len(rest) < n {
			n = len(rest)
		}
		b.WriteString("\r\n ")
		b.WriteString(rest[:n])
		rest = rest[n:]
	}
	return b.String()
}

// icalEscape escapes the RFC-5545 special characters in text property values.
func icalEscape(s string) string {
	r := strings.NewReplacer(
		"\\", "\\\\",
		";", "\\;",
		",", "\\,",
		"\n", "\\n",
	)
	return r.Replace(s)
}

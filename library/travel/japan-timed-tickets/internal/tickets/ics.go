// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package tickets

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type icsEvent struct {
	key     string
	name    string
	start   time.Time
	allDay  string // set for an estimated release window (all-day event)
	lastDay string
	first   string
	last    string
	basis   string
	handoff string
}

// BuildICS renders one calendar event per future sale moment and returns the
// calendar and its event count. Rows that share a sight and moment are merged
// into one event that names the visit dates. Callers should not write a
// calendar with zero events (RFC 5545 needs at least one component).
func BuildICS(rows []OnsaleRow, now time.Time) (string, int) {
	today := DateOf(now).String()
	events := map[string]*icsEvent{}
	for _, r := range rows {
		if r.State != StateOpensAt {
			continue
		}
		var ev icsEvent
		switch {
		case r.OnSaleJST != nil:
			t, err := time.Parse(time.RFC3339, *r.OnSaleJST)
			if err != nil || t.Before(now) {
				continue
			}
			ev = icsEvent{key: r.Sight + "|" + *r.OnSaleJST, start: t}
		case r.OnSaleEarliest != nil:
			ev = icsEvent{key: r.Sight + "|" + *r.OnSaleEarliest, allDay: *r.OnSaleEarliest, lastDay: *r.OnSaleEarliest}
			if r.OnSaleLatest != nil {
				ev.lastDay = *r.OnSaleLatest
			}
			if ev.lastDay < today {
				continue
			}
			if ev.allDay < today {
				ev.allDay = today // the window is open now; remind from today
			}
		default:
			continue
		}
		if cur, ok := events[ev.key]; ok {
			if r.VisitDate < cur.first {
				cur.first = r.VisitDate
			}
			if r.VisitDate > cur.last {
				cur.last = r.VisitDate
			}
			continue
		}
		ev.name, ev.first, ev.last, ev.basis, ev.handoff = r.NameEN, r.VisitDate, r.VisitDate, r.SaleBasis, r.HandoffURL
		e := ev
		events[ev.key] = &e
	}
	keys := make([]string, 0, len(events))
	for k := range events {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	w := func(line string) { b.WriteString(foldICS(line)); b.WriteString("\r\n") }
	w("BEGIN:VCALENDAR")
	w("VERSION:2.0")
	w("PRODID:-//japan-timed-tickets-pp-cli//sale moments//EN")
	w("CALSCALE:GREGORIAN")
	stamp := now.UTC().Format("20060102T150405Z")
	for _, k := range keys {
		ev := events[k]
		sum := sha256.Sum256([]byte(k)) // stable UID per event key; not a security use
		w("BEGIN:VEVENT")
		w("UID:" + hex.EncodeToString(sum[:8]) + "@japan-timed-tickets-pp-cli")
		w("DTSTAMP:" + stamp)
		visits := ev.first
		if ev.last != ev.first {
			visits = ev.first + " to " + ev.last
		}
		if ev.allDay != "" {
			d, _ := ParseDate(ev.allDay)
			w("DTSTART;VALUE=DATE:" + d.Time(0, 0).Format("20060102"))
			w("DTEND;VALUE=DATE:" + d.AddDays(1).Time(0, 0).Format("20060102"))
			w("SUMMARY:" + escICS(fmt.Sprintf("Expected on sale: %s (visits %s)", ev.name, visits)))
			desc := fmt.Sprintf("Estimated release window %s to %s. Basis: %s. Buy: %s", ev.allDay, ev.lastDay, ev.basis, ev.handoff)
			w("DESCRIPTION:" + escICS(desc))
		} else {
			w("DTSTART:" + ev.start.UTC().Format("20060102T150405Z"))
			w("DURATION:PT30M")
			w("SUMMARY:" + escICS(fmt.Sprintf("On sale: %s (visits %s)", ev.name, visits)))
			desc := fmt.Sprintf("Sale opens %s. Basis: %s. Buy: %s", FormatJST(ev.start), ev.basis, ev.handoff)
			w("DESCRIPTION:" + escICS(desc))
		}
		w("URL:" + ev.handoff)
		w("END:VEVENT")
	}
	w("END:VCALENDAR")
	return b.String(), len(keys)
}

// escICS escapes a TEXT value. Remote text is stripped of control and format
// characters first, so a CR or ESC in a notice cannot start a new property.
func escICS(s string) string {
	s = strings.ReplaceAll(StripControl(s), "\t", " ")
	r := strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`)
	return r.Replace(s)
}

// foldICS folds lines longer than 75 octets per RFC 5545.
func foldICS(line string) string {
	if len(line) <= 75 {
		return line
	}
	var b strings.Builder
	count := 0
	for _, r := range line {
		n := len(string(r))
		if count+n > 75 {
			b.WriteString("\r\n ")
			count = 1
		}
		b.WriteRune(r)
		count += n
	}
	return b.String()
}

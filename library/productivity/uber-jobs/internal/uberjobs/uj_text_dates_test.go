// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestStripHTML covers each rewrite rule. Description filters and salary
// parsing run on this output, so a leftover tag or entity is a missed match.
func TestStripHTML(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"whitespace only", "  \n\t ", ""},
		{"plain text", "Hello world", "Hello world"},
		{"br forms", "a<br>b<br/>c<BR />d", "a\nb\nc\nd"},
		{"paragraphs", "<p>First</p><p>Second</p>", "First\nSecond"},
		{"headings and divs", "<h3>Title</h3><div>Body</div>", "Title\nBody"},
		{"list items", "<ul><li>one</li><li aria-level=\"1\">two</li></ul>", "- one\n- two"},
		{"empty list item dropped", "<ul><li></li><li>x</li></ul>", "- x"},
		{"entities", "Fish &amp; Chips &lt;3 &#39;yes&#39; &quot;q&quot;", `Fish & Chips <3 'yes' "q"`},
		{"nbsp collapsed", "a&nbsp;&nbsp; b  c", "a b c"},
		{"inline tags become spaces", "<strong>Bold</strong><em>it</em>", "Bold it"},
		{"comment dropped", "<!-- hidden -->Shown<!--\nmulti\n-->", "Shown"},
		{"script and style dropped", "<p>Keep</p><script>alert('x<y')</script><style>.a{color:red}</style><noscript>enable js</noscript>", "Keep"},
		{"whole document", "<!DOCTYPE html> <html> <head> <title><p> Cleaned Document </p></title> </head> <body> <p> <strong> About the role </strong> </p> <p> Body text. </p></body></html>", "About the role\nBody text."},
		{"head with style after title", "<html><head><title>Doc</title><style>p{}</style></head><body><p>Hi</p></body></html>", "Hi"},
		{"blank lines collapse", "<p>a</p><p></p><p></p><p></p><p>b</p>", "a\n\nb"},
		{"crlf normalized", "a\r\nb\rc", "a\nb\nc"},
		{"tabs collapse", "a\t\t b", "a b"},
	}
	for _, tc := range cases {
		if got := StripHTML(tc.in); got != tc.want {
			t.Errorf("%s: StripHTML(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// TestSentences splits on terminal punctuation, which is what screen's
// evidence output quotes. Line-break splitting is not asserted here: it is
// a reported bug (strings.Fields eats the "\n" markers).
func TestSentences(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"One. Two! Three? Four", []string{"One.", "Two!", "Three?", "Four"}},
		{"  spaced   out   words.  ", []string{"spaced out words."}},
		{"Ends with a period.\nNext line here.", []string{"Ends with a period.", "Next line here."}},
		{"e-mail us! Now? ok", []string{"e-mail us!", "Now?", "ok"}},
		{"", nil},
		{"\n\n", nil},
	}
	for _, tc := range cases {
		if got := Sentences(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Sentences(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestParseSiteDate covers the live DisplayDate shape plus the Oracle shapes.
func TestParseSiteDate(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"2026-10-05T10:05:14Z", "2026-10-05T10:05:14Z", true},
		{"2026-10-02T21:46:44+00:00", "2026-10-02T21:46:44Z", true},
		{"2026-10-02T21:46:44+05:00", "2026-10-02T16:46:44Z", true},
		{"2026-10-05T10:05:14.123Z", "2026-10-05T10:05:14Z", true},
		{"2026-10-05T10:05:14", "2026-10-05T10:05:14Z", true},
		{"2026-10-05", "2026-10-05T00:00:00Z", true},
		{" 2026-10-05 ", "2026-10-05T00:00:00Z", true},
		{"", "", false},
		{"yesterday", "", false},
		{"10/05/2026", "", false},
	}
	for _, tc := range cases {
		got, ok := ParseSiteDate(tc.in)
		if ok != tc.ok {
			t.Errorf("ParseSiteDate(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			continue
		}
		if ok && (got.Format(time.RFC3339) != tc.want || got.Location() != time.UTC) {
			t.Errorf("ParseSiteDate(%q) = %v, want %s in UTC", tc.in, got, tc.want)
		}
	}
}

// TestIsFloorDate: only the exact migration instant is the floor, in any
// offset spelling; nearby instants and the bare date are real dates.
func TestIsFloorDate(t *testing.T) {
	for in, want := range map[string]bool{
		"2026-06-19T07:30:00Z":      true,
		"2026-06-19T07:30:00+00:00": true,
		"2026-06-19T12:30:00+05:00": true,
		"2026-06-19T07:30:01Z":      false,
		"2026-06-19":                false,
		"":                          false,
		"junk":                      false,
	} {
		if got := IsFloorDate(in); got != want {
			t.Errorf("IsFloorDate(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestPostingDatesTrackerFormat: "Month D, YYYY" with no zero padding, and
// an unparseable raw date leaves both fields null without claiming a floor.
func TestPostingDatesTrackerFormat(t *testing.T) {
	on, tracker, floor := postingDates("2026-01-05T00:00:00Z")
	if ujS(on) != "2026-01-05" || ujS(tracker) != "January 5, 2026" || floor {
		t.Errorf("postingDates = %s %s %v", ujS(on), ujS(tracker), floor)
	}
	on, tracker, floor = postingDates("not a date")
	if on != nil || tracker != nil || floor {
		t.Errorf("unparseable date = %s %s %v, want null null false", ujS(on), ujS(tracker), floor)
	}
	on, tracker, floor = postingDates(DateFloor)
	if on != nil || tracker != nil || !floor {
		t.Errorf("floor date = %s %s %v, want null null true", ujS(on), ujS(tracker), floor)
	}
	if TrackerDateLayout != "January 2, 2006" || DateFloor != "2026-06-19T07:30:00Z" {
		t.Errorf("contract constants changed: %q %q", TrackerDateLayout, DateFloor)
	}
}

// TestPostedWithin: bad input fails before any request; zero and negative
// windows are rejected rather than meaning "everything".
func TestPostedWithin(t *testing.T) {
	good := map[string]time.Duration{"": 0, "24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "2w": 14 * 24 * time.Hour, " 90m ": 90 * time.Minute}
	for in, want := range good {
		got, err := PostedWithin(in)
		if err != nil || got != want {
			t.Errorf("PostedWithin(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	bad := map[string]string{"abc": "invalid --posted-within", "1.5d": "invalid --posted-within", "0": "must be positive", "0d": "must be positive", "-1h": "must be positive", "-2w": "must be positive"}
	for in, msg := range bad {
		got, err := PostedWithin(in)
		if err == nil || got != 0 || !strings.Contains(err.Error(), msg) {
			t.Errorf("PostedWithin(%q) = %v, %v; want an error containing %q", in, got, err, msg)
		}
	}
}

func ujDated(raw string) Posting {
	p := Posting{ID: raw, PostedRaw: ujStrp(raw)}
	p.PostedOn, p.PostedDate, p.PostedDateIsFloor = postingDates(raw)
	return p
}

// TestWithinWindowDayFloor: the window opens at 00:00 UTC of the day it
// starts on, inclusive; one second earlier is out. Floor-dated and undated
// postings never match a window.
func TestWithinWindowDayFloor(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour // opens 2026-10-04T12:00Z, floored to 2026-10-04T00:00Z
	cases := []struct {
		raw  string
		want bool
	}{
		{"2026-10-04T00:00:01Z", true},
		{"2026-10-04T00:00:00Z", true},
		{"2026-10-03T23:59:59Z", false},
		{"2026-10-05T11:59:59Z", true},
		{"2026-10-06T09:00:00Z", true}, // future-dated still inside
		{"2026-10-04", true},           // Oracle bare date
		{"2026-10-03", false},
	}
	for _, tc := range cases {
		if got := WithinWindow(ujDated(tc.raw), day, now); got != tc.want {
			t.Errorf("WithinWindow(%s, 24h, %s) = %v, want %v", tc.raw, now.Format(time.RFC3339), got, tc.want)
		}
	}
	floor := ujDated(DateFloor)
	if !floor.PostedDateIsFloor || WithinWindow(floor, 365*day, now) {
		t.Errorf("floor row matched a 365d window that covers its raw date")
	}
	if WithinWindow(Posting{ID: "undated"}, 365*day, now) {
		t.Errorf("undated posting matched a window")
	}
	if WithinWindow(Posting{ID: "junk", PostedRaw: ujStrp("junk")}, 365*day, now) {
		t.Errorf("unparseable date matched a window")
	}
	if !WithinWindow(floor, 0, now) || !WithinWindow(Posting{}, -time.Hour, now) {
		t.Errorf("no window (0 or negative) must match everything")
	}
}

// TestWithinWindowUsesUTCDay: a caller clock in another zone still floors
// to the UTC day, so results do not shift with the user's timezone.
func TestWithinWindowUsesUTCDay(t *testing.T) {
	pkt := time.FixedZone("PKT", 5*3600)
	now := time.Date(2026, 10, 5, 2, 0, 0, 0, pkt) // 2026-10-04T21:00Z; 24h opens 2026-10-03T21:00Z
	if !WithinWindow(ujDated("2026-10-03T00:00:01Z"), 24*time.Hour, now) {
		t.Errorf("posting early on 2026-10-03 UTC should be inside the UTC-day floor")
	}
	if WithinWindow(ujDated("2026-10-02T23:59:59Z"), 24*time.Hour, now) {
		t.Errorf("posting on 2026-10-02 UTC should be outside")
	}
}

// TestSortKeyFloorLast: --sort recent puts real dates newest first and
// floor or undated rows after every dated row.
func TestSortKeyFloorLast(t *testing.T) {
	rows := []Posting{ujDated(DateFloor), ujDated("2026-09-01T00:00:00Z"), {ID: "undated"}, ujDated("2026-10-05T09:15:59Z"), ujDated("2026-06-01T00:00:00Z")}
	sort.SliceStable(rows, func(i, j int) bool { return SortKey(rows[i]).After(SortKey(rows[j])) })
	got := []string{}
	for _, r := range rows {
		got = append(got, r.ID)
	}
	want := []string{"2026-10-05T09:15:59Z", "2026-09-01T00:00:00Z", "2026-06-01T00:00:00Z", DateFloor, "undated"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sorted = %q, want %q", got, want)
	}
	if !SortKey(ujDated(DateFloor)).IsZero() || !SortKey(Posting{}).IsZero() {
		t.Errorf("floor and undated sort keys must be zero")
	}
	if k := SortKey(ujDated("2026-10-05T09:15:59Z")); !k.Equal(time.Date(2026, 10, 5, 9, 15, 59, 0, time.UTC)) {
		t.Errorf("SortKey = %v", k)
	}
}

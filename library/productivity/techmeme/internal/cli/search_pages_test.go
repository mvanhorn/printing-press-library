// Copyright 2026 Dave Morin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// syntheticSearchPage renders a results page in the live markup shape with
// one item per date; links are unique per (offset, index).
func syntheticSearchPage(offset int, dates []string) string {
	var b strings.Builder
	b.WriteString(`<DIV class="resultscanvas"><div class="items">`)
	for i, d := range dates {
		link := fmt.Sprintf("https://example.com/story-%d", offset+i)
		fmt.Fprintf(&b, `<DIV CLASS="item"><CITE><A HREF="https://example.com/">Example</A>:</CITE>`+
			`<STRONG><A HREF="%s">Story %d</A></STRONG>`+
			`<div class="iinf"><SPAN CLASS="idate">%s</SPAN></div></DIV>`, link, offset+i, d)
	}
	b.WriteString(`</div><div class="prevnext"></div></DIV>`)
	return b.String()
}

func repeatDate(d string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = d
	}
	return out
}

type fakeSearchFetch struct {
	pages  map[int]string // offset -> page HTML
	fail   map[int]bool
	starts []string
}

func (f *fakeSearchFetch) fetch(params map[string]string) ([]byte, error) {
	f.starts = append(f.starts, params["start"])
	offset := 0
	if s := params["start"]; s != "" {
		fmt.Sscanf(s, "%d", &offset)
	}
	if f.fail[offset] {
		return nil, errors.New("boom")
	}
	return []byte(f.pages[offset]), nil
}

var searchNow = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

func TestFetchSearchPages_SinglePageRequestUnchanged(t *testing.T) {
	f := &fakeSearchFetch{pages: map[int]string{0: syntheticSearchPage(0, repeatDate("Nov 14, 2026", 10))}}
	results, _, partial, err := fetchSearchPages(f.fetch, "microsoft", 0, 1, 0, searchNow)
	if err != nil || partial != nil {
		t.Fatalf("err=%v partial=%v", err, partial)
	}
	if len(results) != 10 || len(f.starts) != 1 || f.starts[0] != "" {
		t.Fatalf("got %d results, starts=%q; want 10 results and one request without start", len(results), f.starts)
	}
}

func TestFetchSearchPages_ReadsPagesInOrderAndRenumbers(t *testing.T) {
	f := &fakeSearchFetch{pages: map[int]string{
		0:  syntheticSearchPage(0, repeatDate("Nov 14, 2026", 10)),
		10: syntheticSearchPage(10, repeatDate("Nov 10, 2026", 10)),
		20: syntheticSearchPage(20, repeatDate("Nov 5, 2026", 10)),
	}}
	results, _, _, err := fetchSearchPages(f.fetch, "microsoft", 0, 3, 0, searchNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 30 {
		t.Fatalf("got %d results, want 30", len(results))
	}
	for i, r := range results {
		if r.Num != i+1 || r.Headline != fmt.Sprintf("Story %d", i) {
			t.Fatalf("result %d = %+v", i, r)
		}
	}
	if strings.Join(f.starts, ",") != ",10,20" {
		t.Fatalf("starts = %q", f.starts)
	}
}

func TestFetchSearchPages_StopsAtShortLastPage(t *testing.T) {
	f := &fakeSearchFetch{pages: map[int]string{
		0:  syntheticSearchPage(0, repeatDate("Nov 14, 2026", 10)),
		10: syntheticSearchPage(10, repeatDate("Nov 10, 2026", 4)),
	}}
	results, _, _, _ := fetchSearchPages(f.fetch, "microsoft", 0, 5, 0, searchNow)
	if len(results) != 14 || len(f.starts) != 2 {
		t.Fatalf("got %d results over %d requests, want 14 over 2", len(results), len(f.starts))
	}
}

func TestFetchSearchPages_DaysStopsOncePastTheWindow(t *testing.T) {
	// Page 2 reaches back before the 30-day window; page 3 is never requested.
	page2 := append(repeatDate("Oct 20, 2026", 5), repeatDate("Oct 1, 2026", 5)...)
	f := &fakeSearchFetch{pages: map[int]string{
		0:  syntheticSearchPage(0, repeatDate("Nov 14, 2026", 10)),
		10: syntheticSearchPage(10, page2),
		20: syntheticSearchPage(20, repeatDate("Sep 1, 2026", 10)),
	}}
	results, _, _, _ := fetchSearchPages(f.fetch, "microsoft", 0, 5, 30, searchNow)
	if len(f.starts) != 2 {
		t.Fatalf("requests = %q, want 2", f.starts)
	}
	if got := len(filterSearchByDays(results, 30, searchNow)); got != 15 {
		t.Fatalf("in-window results = %d, want 15", got)
	}
}

func TestFetchSearchPages_StartOffsetAndDuplicateLinks(t *testing.T) {
	// The second page repeats the first page's last link (results shifted
	// between requests); it is kept once.
	dup := syntheticSearchPage(29, repeatDate("Nov 1, 2026", 10))
	f := &fakeSearchFetch{pages: map[int]string{
		20: syntheticSearchPage(20, repeatDate("Nov 3, 2026", 10)),
		30: dup,
	}}
	results, _, _, _ := fetchSearchPages(f.fetch, "microsoft", 20, 2, 0, searchNow)
	if strings.Join(f.starts, ",") != "20,30" {
		t.Fatalf("starts = %q", f.starts)
	}
	if len(results) != 19 {
		t.Fatalf("got %d results, want 19 after dropping the repeated link", len(results))
	}
}

func TestFetchSearchPages_LaterPageFailureKeepsEarlierResults(t *testing.T) {
	f := &fakeSearchFetch{
		pages: map[int]string{0: syntheticSearchPage(0, repeatDate("Nov 14, 2026", 10))},
		fail:  map[int]bool{10: true},
	}
	results, _, partial, err := fetchSearchPages(f.fetch, "microsoft", 0, 3, 0, searchNow)
	if err != nil || partial == nil || len(results) != 10 {
		t.Fatalf("err=%v partial=%v results=%d; want nil err, a partial error, 10 results", err, partial, len(results))
	}
}

func TestFetchSearchPages_FirstPageFailureIsAnError(t *testing.T) {
	f := &fakeSearchFetch{fail: map[int]bool{0: true}}
	if _, _, _, err := fetchSearchPages(f.fetch, "microsoft", 0, 3, 0, searchNow); err == nil {
		t.Fatal("want an error when the first page fails")
	}
}

func TestSearchPagesFlagBounds(t *testing.T) {
	for _, args := range [][]string{{"--pages", "0"}, {"--pages", "11"}, {"--start", "-1"}} {
		flags := &rootFlags{}
		cmd := newSearchCmd(flags)
		cmd.SetArgs(append([]string{"microsoft"}, args...))
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		if err := cmd.Execute(); err == nil {
			t.Fatalf("%v: want a usage error", args)
		}
	}
}

// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Regression tests for the bugs the Phase 3 test pass found and fixed
// (2026-10-05). Each one failed before its fix.

package uberjobs

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

// TestSearchAllPageTotalDisagreesWithProbe: the probe said 56 but the page
// came back empty with totalJobs 0; that read must not be complete, or a
// full sync closes every stored posting.
func TestSearchAllPageTotalDisagreesWithProbe(t *testing.T) {
	rows := ujCorpusRows(t)
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pagesize") == "1" {
			_, _ = w.Write(ujSearchJSON(rows[:1], 56))
			return
		}
		_, _ = w.Write(ujSearchJSON(nil, 0))
	})
	c := ujClient(t, srv.URL)
	res, err := c.SearchAll(context.Background(), Query{})
	if err != nil {
		return // a *ContentError is also an acceptable fix
	}
	if res.Complete {
		t.Fatalf("probe total 56, page 0 rows/totalJobs 0, yet Complete=true (rows=%d, note=%q)", len(res.Rows), res.Note)
	}
}

// TestLatchedRefusalMalformedUntilFailsClosed: a latch whose JSON parses
// but whose until is not RFC3339 is unreadable and must fail closed.
func TestLatchedRefusalMalformedUntilFailsClosed(t *testing.T) {
	dir := t.TempDir()
	raw := `{"host":"jobs.uber.com","status":403,"at":"2026-10-05T08:00:00Z","until":"2026-10-06 00:00:00"}`
	if err := os.WriteFile(latchPath(dir, "jobs.uber.com"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if r := LatchedRefusal(dir, "jobs.uber.com", time.Now()); r == nil || !r.Latched {
		t.Fatalf("latch with an unparseable until let requests through; want fail closed")
	}
}

// TestSentencesSplitsOnLineBreaks: StripHTML emits one line per list item;
// a bullet list without terminal punctuation must not merge into one
// run-on evidence sentence.
func TestSentencesSplitsOnLineBreaks(t *testing.T) {
	got := Sentences("What You'll Need\n- Fluent in both Arabic and English\nPreferred Qualifications")
	want := []string{"What You'll Need", "- Fluent in both Arabic and English", "Preferred Qualifications"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Sentences = %q, want %q", got, want)
	}
}

// TestCacheSkipsRepliesThatFailContentCheck: a 200 maintenance page fails
// the content check and must not be served from cache on the next call.
func TestCacheSkipsRepliesThatFailContentCheck(t *testing.T) {
	first := make(chan struct{}, 1)
	first <- struct{}{}
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-first:
			_, _ = w.Write([]byte("<html>maintenance</html>"))
		default:
			_, _ = w.Write(ujSearchJSON(nil, 0))
		}
	})
	c := ujClient(t, srv.URL)
	c.CacheDir = t.TempDir()
	if _, _, err := c.ProbeTotal(context.Background(), Query{Team: "Sales"}); err == nil {
		t.Fatal("first probe: want a content error")
	}
	if _, _, err := c.ProbeTotal(context.Background(), Query{Team: "Sales"}); err != nil {
		t.Fatalf("second probe was served the cached maintenance page: %v (server requests=%d)", err, srv.Count())
	}
}

// TestParseSalaryRangesLocationStartsAtNearestFor: text before the
// "For <place>-based roles:" clause must not leak into the location.
func TestParseSalaryRangesLocationStartsAtNearestFor(t *testing.T) {
	got := ParseSalaryRanges("You will be eligible for a bonus. For Seattle, WA-based roles: The base salary range for this role is USD $171,000 per year - USD $190,000 per year.")
	if len(got) != 1 || ujS(got[0].Location) != "Seattle, WA" {
		t.Fatalf("ranges = %d, location = %q, want one range for \"Seattle, WA\"", len(got), func() string {
			if len(got) == 0 {
				return ""
			}
			return ujS(got[0].Location)
		}())
	}
}

// TestMatchLocalKeywordDoesNotSpanFields: the offline keyword must match
// inside one field, not across the title/team join.
func TestMatchLocalKeywordDoesNotSpanFields(t *testing.T) {
	p := Posting{ID: "155304", Title: "Senior Engineering Manager - Merchant Fulfillment", JobCategory: ujStrp("Engineer"), SubTeam: ujStrp("Software Engineering"), Description: ujStrp("Plain words only.")}
	if (Filters{Query: "fulfillment engineer"}).MatchLocal(p, 0, time.Now()) {
		t.Fatalf(`"fulfillment engineer" is in no single field but matched across the title/team boundary`)
	}
}

// TestSendRechecksCacheAfterGateWait: concurrent commands queue at the gate,
// and the first one through fills the response cache. A request that waited
// must use that reply instead of fetching the same URL again (the live check
// on 2026-10-05 fetched the 4 MB corpus three times without this).
func TestSendRechecksCacheAfterGateWait(t *testing.T) {
	srv := ujNewFake(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ujSearchJSON(nil, 0))
	})
	c := ujClient(t, srv.URL)
	c.CacheDir = t.TempDir()
	dir := t.TempDir()
	c.Gate = NewGate(dir)
	if err := os.WriteFile(c.Gate.lastPath(), []byte(fmt.Sprint(time.Now().UnixNano())), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := srv.URL + "/api/jobs/search/?page=1&pagesize=1"
	c.Gate.sleep = func(context.Context, time.Duration) error {
		// While this request waits, another process caches the reply.
		c.writeCache(probe, "application/json", &response{Body: ujSearchJSON(nil, 7), Header: http.Header{}, Status: http.StatusOK, URL: probe})
		return nil
	}
	total, resp, err := c.ProbeTotal(context.Background(), Query{})
	if err != nil {
		t.Fatal(err)
	}
	if srv.Count() != 0 || !resp.CacheHit || total != 7 {
		t.Fatalf("after the gate wait: requests=%d cacheHit=%v total=%d, want the cached reply (0 requests, total 7)", srv.Count(), resp.CacheHit, total)
	}
}

// TestSentencesKeepAbbreviationsAndInitials: evidence sentences were cut at
// "Sr." (live screen output, 2026-10-05); abbreviations and initials must not
// end a sentence, while real sentence ends still split.
func TestSentencesKeepAbbreviationsAndInitials(t *testing.T) {
	got := Sentences("The team is adding a detail-oriented Sr. Analyst, e.g. for O2C work. J. Smith leads it! Apply now.")
	want := []string{"The team is adding a detail-oriented Sr. Analyst, e.g. for O2C work.", "J. Smith leads it!", "Apply now."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Sentences = %q, want %q", got, want)
	}
}

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

// Hand-authored, beside the generated tests in this package.
//
// ctgovFetch and ctgovCount reach the network through the ctgovClient
// interface, and everything that happens when that call goes wrong was
// unguarded. Mutation-verified against the tests that existed before this
// file: four separate changes to intel.go each left the package green —
//
//	return out, err           -> return nil, err   (partial results dropped)
//	if maxPages <= 0 { ... }  -> removed           (zero-page guard gone)
//	if len(out) >= maxStudies -> removed           (result cap gone)
//	return 0, err             -> return 0, nil     (count error swallowed)
//
// The control was changing the returned count by one, which DID fail three
// existing tests, so the run is sensitive to this file and the four above are
// a real gap rather than a dead target.
//
// The last of them is the one with teeth. Many call sites take a number from
// ctgovCount and print it as a study total. A swallowed error there does not
// surface as a failure: it renders a confident zero, which reads as "nothing
// matched" rather than "the lookup broke".
//
// These tests pin the CURRENT behaviour. Returning partial results alongside
// an error is unusual enough to be worth stating explicitly rather than
// leaving the next reader to rediscover it from the source.

// errCtgovProbe is the sentinel the fake clients return, so a test can assert
// the caller got THIS error rather than merely some error.
var errCtgovProbe = errors.New("ctgov probe failure")

// pagingCtgov is a ctgovClient that serves a scripted sequence of pages and
// can fail on a chosen one. It is deliberately separate from mockCtgov in
// compare_test.go: that one answers every fetch with the same fixed study set
// and never fails, which is right for the aggregation tests there and cannot
// express either paging or an error.
type pagingCtgov struct {
	// pages are served in order, one per Get. Every page except the last
	// carries a nextPageToken, so a fetch that honours the cursor keeps going.
	pages [][]json.RawMessage
	// failOn is the zero-based index of the page that returns an error
	// instead of a body. Negative means never fail.
	failOn int
	// calls counts Get invocations, which is how a test tells "stopped after
	// two pages" from "asked for five and ignored three".
	calls int
	// tokens records the pageToken sent with each call, empty for the first.
	// A fetch that forgot to pass the cursor would re-serve page one forever,
	// and this makes that visible.
	tokens []string
}

func (p *pagingCtgov) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	idx := p.calls
	p.calls++
	p.tokens = append(p.tokens, params["pageToken"])

	if p.failOn >= 0 && idx == p.failOn {
		return nil, errCtgovProbe
	}
	if idx >= len(p.pages) {
		// Past the script: an empty page with no cursor, which is what the
		// real API returns once a query runs out of results.
		return json.Marshal(map[string]any{"studies": []json.RawMessage{}})
	}

	body := map[string]any{"studies": p.pages[idx]}
	if idx < len(p.pages)-1 {
		body["nextPageToken"] = fmt.Sprintf("token-%d", idx+1)
	}
	return json.Marshal(body)
}

// countingCtgov answers count queries and can fail. Kept separate from
// pagingCtgov so a count test cannot accidentally depend on paging behaviour.
type countingCtgov struct {
	total int
	err   error
}

func (c *countingCtgov) Get(_ context.Context, _ string, _ map[string]string) (json.RawMessage, error) {
	if c.err != nil {
		return nil, c.err
	}
	return json.Marshal(map[string]any{"totalCount": c.total})
}

// ctgovTestPage builds one page of n studies with distinct NCT ids derived
// from a prefix, so an assertion can name which trial it expected.
func ctgovTestPage(prefix string, n int) []json.RawMessage {
	out := make([]json.RawMessage, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, studyJSON(fmt.Sprintf("NCT%s%03d", prefix, i), "RECRUITING", "PHASE2", "Sponsor", "INDUSTRY"))
	}
	return out
}

// TestFetchKeepsPartialResultsOnError is the surprising one, and the reason
// this file states it rather than leaving it implicit. A fetch that fails on
// its third page does NOT discard the two pages it already has: the caller
// receives both the trials and the error, and decides what to do with them.
func TestFetchKeepsPartialResultsOnError(t *testing.T) {
	c := &pagingCtgov{
		pages:  [][]json.RawMessage{ctgovTestPage("A", 2), ctgovTestPage("B", 2), ctgovTestPage("C", 2)},
		failOn: 2,
	}

	got, err := ctgovFetch(context.Background(), c, map[string]string{}, 100, 5)

	if !errors.Is(err, errCtgovProbe) {
		t.Fatalf("err = %v, want the client's own error to reach the caller", err)
	}
	if len(got) != 4 {
		t.Errorf("got %d trials, want the 4 from the two pages that succeeded", len(got))
	}
}

// TestFetchPropagatesAnImmediateError covers the failure that happens before
// anything has been collected: the error still travels, and the result is
// empty rather than partially populated.
func TestFetchPropagatesAnImmediateError(t *testing.T) {
	c := &pagingCtgov{pages: [][]json.RawMessage{ctgovTestPage("A", 3)}, failOn: 0}

	got, err := ctgovFetch(context.Background(), c, map[string]string{}, 100, 5)

	if !errors.Is(err, errCtgovProbe) {
		t.Fatalf("err = %v, want the client's own error", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d trials from a fetch whose first call failed, want 0", len(got))
	}
}

// TestFetchStopsAtMaxStudies pins the result cap. The cap is applied AFTER
// normalization, so it counts trials that made it into the output rather than
// studies the API returned.
func TestFetchStopsAtMaxStudies(t *testing.T) {
	// The first page mixes one study normalizeStudy rejects into four it
	// accepts. The cap counts trials that reached the output, not studies the
	// API returned, so a rejected study must not consume a slot: asking for 3
	// from this page has to yield 3 real trials.
	const unnormalizable = `{"protocolSection":{}}`
	mixed := append([]json.RawMessage{json.RawMessage(unnormalizable)}, ctgovTestPage("A", 4)...)
	c := &pagingCtgov{
		pages:  [][]json.RawMessage{mixed, ctgovTestPage("B", 5)},
		failOn: -1,
	}

	got, err := ctgovFetch(context.Background(), c, map[string]string{}, 3, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d trials, want exactly the 3 asked for", len(got))
	}
	// Reaching the cap mid-page returns immediately, so the second page is
	// never requested. Without that, a small limit over a large corpus would
	// still pay for every page.
	if c.calls != 1 {
		t.Errorf("client was called %d times, want 1: the cap should stop the scan", c.calls)
	}
}

// TestFetchStopsAtMaxPages is the other half of the bound, and the one a
// reader is most likely to get wrong: the scan stops at the page limit EVEN
// WHEN the response carries a cursor saying more results exist.
func TestFetchStopsAtMaxPages(t *testing.T) {
	c := &pagingCtgov{
		pages: [][]json.RawMessage{
			ctgovTestPage("A", 2), ctgovTestPage("B", 2),
			ctgovTestPage("C", 2), ctgovTestPage("D", 2),
		},
		failOn: -1,
	}

	got, err := ctgovFetch(context.Background(), c, map[string]string{}, 100, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.calls != 2 {
		t.Errorf("client was called %d times, want 2: maxPages bounds the scan", c.calls)
	}
	if len(got) != 4 {
		t.Errorf("got %d trials, want the 4 on the first two pages", len(got))
	}
	// The cursor must be sent AND be the one the previous page handed back.
	// Checking only that it is non-empty would keep the test green if a change
	// forwarded a stale or wrong token, which the real API would reject.
	if len(c.tokens) != 2 {
		t.Fatalf("recorded %d pageToken values, want 2", len(c.tokens))
	}
	if c.tokens[0] != "" {
		t.Errorf("first call carried pageToken %q, want none", c.tokens[0])
	}
	if c.tokens[1] != "token-1" {
		t.Errorf("second call carried pageToken %q, want token-1 from the first page", c.tokens[1])
	}
}

// TestFetchTreatsNonPositiveMaxPagesAsOne guards the guard. Without the
// floor, a zero or negative page limit makes the loop body never run, and the
// fetch returns no trials and no error — an empty report that looks like a
// legitimate "nothing found".
func TestFetchTreatsNonPositiveMaxPagesAsOne(t *testing.T) {
	for _, maxPages := range []int{0, -1} {
		c := &pagingCtgov{pages: [][]json.RawMessage{ctgovTestPage("A", 2)}, failOn: -1}

		got, err := ctgovFetch(context.Background(), c, map[string]string{}, 100, maxPages)
		if err != nil {
			t.Fatalf("maxPages=%d: unexpected error: %v", maxPages, err)
		}
		if c.calls != 1 {
			t.Errorf("maxPages=%d: client called %d times, want 1", maxPages, c.calls)
		}
		if len(got) != 2 {
			t.Errorf("maxPages=%d: got %d trials, want 2", maxPages, len(got))
		}
	}
}

// TestFetchHandlesAnEmptyResponse is the quiet case: a query that matches
// nothing is not an error, and must not be reported as one.
func TestFetchHandlesAnEmptyResponse(t *testing.T) {
	c := &pagingCtgov{pages: [][]json.RawMessage{{}}, failOn: -1}

	got, err := ctgovFetch(context.Background(), c, map[string]string{}, 100, 5)
	if err != nil {
		t.Fatalf("an empty result set is not an error, got: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d trials from an empty response, want 0", len(got))
	}
	if got == nil {
		t.Error("returned a nil slice: callers range over this and marshal it")
	}
}

// TestCountPropagatesTheClientError is the one that matters most in the
// output a reader sees. Many call sites take a number from ctgovCount and
// several print it directly. If the error were swallowed, a broken lookup
// would render as a confident "0 studies" instead of a failure.
func TestCountPropagatesTheClientError(t *testing.T) {
	c := &countingCtgov{err: errCtgovProbe}

	got, err := ctgovCount(context.Background(), c, map[string]string{})

	if !errors.Is(err, errCtgovProbe) {
		t.Fatalf("err = %v, want the client's own error to reach the caller", err)
	}
	if got != 0 {
		t.Errorf("count = %d alongside an error, want 0", got)
	}
}

// TestCountReturnsTheTotal is the success half, so the error test above
// cannot pass by the function being broken in both directions.
func TestCountReturnsTheTotal(t *testing.T) {
	c := &countingCtgov{total: 42}

	got, err := ctgovCount(context.Background(), c, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 {
		t.Errorf("count = %d, want 42", got)
	}
}

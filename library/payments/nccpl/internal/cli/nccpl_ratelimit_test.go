package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/cliutil"
)

// flowsTestResource mirrors the real `flows` entry: envelope key "d", the shape
// scstrade actually returns.
func flowsTestResource(t *testing.T) nccplResource {
	t.Helper()
	res, ok := nccplResourceByName("flows")
	if !ok {
		t.Fatal("flows resource is not registered")
	}
	if res.Envelope != "d" {
		t.Fatalf("flows envelope = %q, want \"d\"", res.Envelope)
	}
	return res
}

// A 429 must arrive as a typed *cliutil.RateLimitError carrying the upstream's
// Retry-After. Before this, every non-200 collapsed into "flows <date>: HTTP %d",
// which made a throttle indistinguishable from a bad date and left the caller's
// retry loop no way to tell "wait" from "this will never work".
func TestNCCPLFetchFlowsRateLimitReturnsTypedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("slow down"))
	}))
	defer server.Close()

	_, err := nccplFetchFlows(context.Background(), server.Client(), server.URL, flowsTestResource(t), "2026-09-03")
	var throttled *cliutil.RateLimitError
	if !errors.As(err, &throttled) {
		t.Fatalf("err = %v, want errors.As *cliutil.RateLimitError", err)
	}
	if throttled.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %s, want 30s", throttled.RetryAfter)
	}
	if throttled.URL != server.URL {
		t.Errorf("URL = %q, want %q", throttled.URL, server.URL)
	}
	// The date has to survive the wrap, or the report cannot say which
	// observation is missing.
	if !strings.Contains(err.Error(), "2026-09-03") {
		t.Errorf("error %q does not name the date", err)
	}
}

// No Retry-After is still a throttle. The caller must get a usable delay rather
// than zero, which would busy-loop straight back into the limit.
func TestNCCPLFetchFlowsRateLimitWithoutRetryAfterHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := nccplFetchFlows(context.Background(), server.Client(), server.URL, flowsTestResource(t), "2026-09-03")
	var throttled *cliutil.RateLimitError
	if !errors.As(err, &throttled) {
		t.Fatalf("err = %v, want errors.As *cliutil.RateLimitError", err)
	}
	if throttled.RetryAfter <= 0 {
		t.Errorf("RetryAfter = %s, want a positive default", throttled.RetryAfter)
	}
}

// Regression guard on the other side of the fix: ordinary failures must NOT be
// reclassified as throttles, or an abort would fire on a permanent error and the
// walk would stop for a reason that will never clear.
func TestNCCPLFetchFlowsNonThrottleErrorStaysUntyped(t *testing.T) {
	for _, code := range []int{http.StatusInternalServerError, http.StatusBadRequest, http.StatusForbidden} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
		}))
		_, err := nccplFetchFlows(context.Background(), server.Client(), server.URL, flowsTestResource(t), "2026-09-03")
		server.Close()
		if err == nil {
			t.Fatalf("HTTP %d: want an error", code)
		}
		var throttled *cliutil.RateLimitError
		if errors.As(err, &throttled) {
			t.Errorf("HTTP %d was classified as a rate limit: %v", code, err)
		}
	}
}

// The 429 branch sits in front of the success path, so cover the success path too.
func TestNCCPLFetchFlowsSuccessParsesEnvelope(t *testing.T) {
	var gotBody bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = gotBody.ReadFrom(r.Body)
		_, _ = w.Write([]byte(`{"d":[{"FLSectorName":"BANKS / DFI","FLTypeNew":"FIPI","FLNetValueUSD":0.64}]}`))
	}))
	defer server.Close()

	rows, err := nccplFetchFlows(context.Background(), server.Client(), server.URL, flowsTestResource(t), "2026-09-03")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	// The upstream takes US-format dates; sending ISO would silently return the
	// wrong day.
	if !strings.Contains(gotBody.String(), `"date1":"09/03/2026"`) {
		t.Errorf("request body %q does not carry the MM/DD/YYYY date", gotBody.String())
	}
}

// The remainder a throttled walk reports has to be the exact set of dates it
// still owes: the throttled date itself (attempted, not committed), plus every
// date after it that is not already in the ledger. Nothing invented, nothing
// dropped, original order preserved.
func TestNCCPLUnfetchedDatesReturnsExactRemainder(t *testing.T) {
	dates := []string{"2026-09-01", "2026-09-02", "2026-09-03", "2026-09-04", "2026-09-07"}
	stored := map[string]bool{"2026-09-01": true, "2026-09-04": true}

	// Throttled while fetching index 2.
	got := nccplUnfetchedDates(dates[2:], stored)
	want := []string{"2026-09-03", "2026-09-07"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unfetched = %v, want %v", got, want)
	}
	// Every reported date must come from the requested set -- never synthesized.
	requested := map[string]bool{}
	for _, d := range dates {
		requested[d] = true
	}
	for _, d := range got {
		if !requested[d] {
			t.Errorf("unfetched contains %q, which was never requested", d)
		}
		if stored[d] {
			t.Errorf("unfetched contains %q, which is already committed", d)
		}
	}
}

func TestNCCPLUnfetchedDatesAllStoredIsEmpty(t *testing.T) {
	stored := map[string]bool{"2026-09-03": true, "2026-09-04": true}
	if got := nccplUnfetchedDates([]string{"2026-09-03", "2026-09-04"}, stored); len(got) != 0 {
		t.Fatalf("unfetched = %v, want empty", got)
	}
}

// A throttled run must exit non-zero (7) so an unattended caller can tell partial
// coverage from a clean run, and the message must say how many dates are missing.
func TestNCCPLRateLimitAbortErrUsesRateLimitExitCode(t *testing.T) {
	err := nccplRateLimitAbortErr("flows", "30s", 2600)
	if got := ExitCode(err); got != 7 {
		t.Errorf("ExitCode = %d, want 7", got)
	}
	for _, want := range []string{"flows", "429", "2600", "30s"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestNCCPLSyncThrottleErr(t *testing.T) {
	clean := nccplSyncView{Resources: []nccplSyncResourceResult{{Resource: "fipi", Fetched: 3}}}
	if err := nccplSyncThrottleErr(clean); err != nil {
		t.Fatalf("clean run returned %v, want nil", err)
	}

	throttled := nccplSyncView{
		RateLimited:           true,
		ResourcesNotAttempted: []string{"lipi", "mts"},
		Resources: []nccplSyncResourceResult{{
			Resource: "fipi", Fetched: 3, RateLimited: true, RetryAfter: "45s",
			Unfetched: []string{"2026-09-03", "2026-09-04"},
		}},
	}
	err := nccplSyncThrottleErr(throttled)
	if err == nil {
		t.Fatal("throttled run returned nil, want an error")
	}
	if got := ExitCode(err); got != 7 {
		t.Errorf("ExitCode = %d, want 7", got)
	}
	// Both kinds of gap have to be visible: dates skipped inside a resource and
	// resources never attempted at all.
	for _, want := range []string{"2 date(s) unfetched", "2 resource(s) not attempted", "45s"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// The report is the owner-facing half of the fix: a missed observation is the
// correct outcome, but only if the run says plainly which ones were missed.
func TestNCCPLPrintUnfetchedStatesTheGap(t *testing.T) {
	var buf bytes.Buffer
	nccplPrintUnfetched(&buf, "flows", "30s",
		[]string{"2026-09-03", "2026-09-04"}, "2026-09-03", "2026-09-04",
		"nccpl-pp-cli flows --from 2026-09-03 --to 2026-09-04")
	out := buf.String()
	for _, want := range []string{
		"rate limit", "429", "30s",
		"committed",                // what survived
		"2 date(s)",                // the size of the gap
		"2026-09-03", "2026-09-04", // the gap itself, named
		"not estimated", // and that it was not filled in
		"resume with",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report does not mention %q:\n%s", want, out)
		}
	}
}

// Long remainders are elided for readability, but the elision must always be
// labelled with the exact count omitted -- a silently truncated list would
// understate the gap.
func TestNCCPLSummarizeDatesLabelsElision(t *testing.T) {
	short := []string{"2026-09-03", "2026-09-04"}
	if got := nccplSummarizeDates(short); got != "2026-09-03 2026-09-04" {
		t.Errorf("short list = %q, want it printed in full", got)
	}

	long := make([]string, 0, 2600)
	for i := 0; i < 2600; i++ {
		long = append(long, time.Date(2016, 8, 1, 0, 0, 0, 0, time.UTC).
			AddDate(0, 0, i).Format("2006-01-02"))
	}
	got := nccplSummarizeDates(long)
	if !strings.Contains(got, long[0]) || !strings.Contains(got, long[len(long)-1]) {
		t.Errorf("summary %q drops the endpoints of the range", got)
	}
	if !strings.Contains(got, "2588 more") {
		t.Errorf("summary %q does not state how many dates were elided", got)
	}
}

// The generic helper the bare-http.Client call sites share.
func TestRateLimitFromResponseNilResponse(t *testing.T) {
	err := cliutil.RateLimitFromResponse(nil, "https://example.test/x")
	if err.RetryAfter <= 0 {
		t.Errorf("RetryAfter = %s, want a positive default", err.RetryAfter)
	}
	if err.URL != "https://example.test/x" {
		t.Errorf("URL = %q", err.URL)
	}
}

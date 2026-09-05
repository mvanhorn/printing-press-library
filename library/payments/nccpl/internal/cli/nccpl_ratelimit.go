package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// Throttle handling for the date-walking fetch loops.
//
// `sync` and `flows` both walk one settlement date per request -- `flows --full`
// covers roughly 2,600 weekdays back to 2016-08-01. Treating an upstream 429 as
// an ordinary per-date failure turns a single throttle into thousands of
// rapid-fire requests against a host that has just asked us to stop, and reports
// the result as nothing but a large failed= count. The loops instead abort on the
// first typed rate-limit error and state exactly what they still owe.
//
// What the abort guarantees, in order of importance:
//
//  1. Everything already fetched stays committed. SaveNCCPLDate writes one date
//     per transaction, so dates stored before the throttle are durable and the
//     abort never rolls them back.
//  2. The dates that were NOT fetched are named, exactly. Not a range that
//     silently glosses over dates already in the ledger -- the precise remainder.
//  3. Nothing is invented to fill the hole. A missed observation stays missed
//     until a later run fetches it for real; no interpolation, no carry-forward,
//     no back-dated stand-in.

// nccplMaxPrintedDates caps how many unfetched dates the human-readable report
// spells out. The full, exact list always goes to --json; this only keeps a
// 2,600-date remainder from burying the rest of the report in a terminal.
const nccplMaxPrintedDates = 12

// nccplUnfetchedDates returns the dates in remaining that are not already in the
// coverage ledger -- the exact set a throttled walk still owes.
//
// Callers pass the slice starting at the date that was throttled, so that date is
// included: it was attempted and not committed. Dates the walk had already
// skipped as stored are excluded, because those are not owed.
func nccplUnfetchedDates(remaining []string, stored map[string]bool) []string {
	out := make([]string, 0, len(remaining))
	for _, d := range remaining {
		if stored[d] {
			continue
		}
		out = append(out, d)
	}
	return out
}

// nccplSummarizeDates renders a date list for human output, eliding the middle
// when it is too long to be useful. The elision is always labelled with the
// exact number omitted so the reader is never left guessing at the size.
func nccplSummarizeDates(dates []string) string {
	if len(dates) <= nccplMaxPrintedDates {
		return strings.Join(dates, " ")
	}
	half := nccplMaxPrintedDates / 2
	return fmt.Sprintf("%s ... %d more ... %s",
		strings.Join(dates[:half], " "),
		len(dates)-nccplMaxPrintedDates,
		strings.Join(dates[len(dates)-half:], " "))
}

// nccplPrintUnfetched states plainly what a throttled walk did and did not store.
// It is written to stderr so it survives a piped --json stdout.
func nccplPrintUnfetched(w io.Writer, resource, retryAfter string, unfetched []string, from, to, resume string) {
	fmt.Fprintf(w, "\n%s: stopped by upstream rate limit (HTTP 429)", resource)
	if retryAfter != "" {
		fmt.Fprintf(w, "; retry after %s", retryAfter)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  committed: every date fetched before the throttle is already stored.")
	if len(unfetched) == 0 {
		fmt.Fprintln(w, "  unfetched: none.")
		return
	}
	fmt.Fprintf(w, "  unfetched: %d date(s), %s .. %s -- not stored, and not estimated.\n", len(unfetched), from, to)
	fmt.Fprintf(w, "  dates: %s\n", nccplSummarizeDates(unfetched))
	if resume != "" {
		fmt.Fprintf(w, "  resume with: %s\n", resume)
	}
}

// nccplRateLimitAbortErr builds the exit-code-7 error a throttled walk returns
// after it has already printed its report, so the run reports partial coverage
// through both the output and the exit status.
func nccplRateLimitAbortErr(what, retryAfter string, unfetched int) error {
	msg := fmt.Sprintf("%s: upstream rate limited (HTTP 429); stopped with %d date(s) unfetched", what, unfetched)
	if retryAfter != "" {
		msg += "; retry after " + retryAfter
	}
	return rateLimitErr(errors.New(msg))
}

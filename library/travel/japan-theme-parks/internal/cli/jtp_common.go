// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/parks"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/sources"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/store"
)

// sourceRef is one source entry in a response meta block.
type sourceRef struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	URL         string `json:"url"`
	FetchedAt   string `json:"fetched_at,omitempty"`
	Attribution string `json:"attribution,omitempty"`
	Link        string `json:"attribution_url,omitempty"`
	Note        string `json:"note,omitempty"`
}

type fetchFailure struct {
	Source string `json:"source"`
	URL    string `json:"url,omitempty"`
	Error  string `json:"error"`
}

type respMeta struct {
	GeneratedAt   string         `json:"generated_at"`
	Timezone      string         `json:"timezone"`
	Sources       []sourceRef    `json:"sources"`
	FetchFailures []fetchFailure `json:"fetch_failures,omitempty"`
	Notes         []string       `json:"notes,omitempty"`
}

func newMeta() respMeta {
	return respMeta{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Timezone: "Asia/Tokyo", Sources: []sourceRef{}}
}

func queueTimesRef(f sources.Fetch) sourceRef {
	return sourceRef{
		Name: parks.SourceQueueTimes, Kind: "community live wait feed (park-published waits, collected every 5 minutes)",
		URL: f.URL, FetchedAt: rfc(f.FetchedAt),
		Attribution: parks.QueueTimesAttribution, Link: parks.QueueTimesURL,
	}
}

func tdrRef(f sources.Fetch) sourceRef {
	return sourceRef{Name: parks.SourceTDR, Kind: "official Tokyo Disney Resort ticket calendar", URL: f.URL, FetchedAt: rfc(f.FetchedAt)}
}

func themeParksRef(f sources.Fetch) sourceRef {
	return sourceRef{
		Name: parks.SourceThemeParks, Kind: "third-party aggregator (not official)", URL: f.URL, FetchedAt: rfc(f.FetchedAt),
		Attribution: parks.ThemeParksAttribution, Link: parks.ThemeParksURL,
	}
}

func rfc(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func jst(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(parks.Tokyo).Format(time.RFC3339)
}

// staleHistoryHint writes a stderr hint when the newest stored snapshot is
// older than --max-age. History is still used; the hint only says how old it is.
func staleHistoryHint(w io.Writer, flags *rootFlags, newest time.Time) {
	if flags == nil || flags.maxAge <= 0 || newest.IsZero() {
		return
	}
	age := time.Since(newest)
	if age <= flags.maxAge {
		return
	}
	fmt.Fprintf(w, "hint: newest local snapshot is %s old (older than --max-age %s); run 'japan-theme-parks-pp-cli snapshot' to add fresh samples\n",
		age.Round(time.Minute), flags.maxAge)
}

// fetchTally records source failures for one command: each failure goes
// to meta.fetch_failures, and allFailed maps a total failure to the typed
// exit code (7 only when every failure was a 429, else 5).
type fetchTally struct {
	meta *respMeta
	errs []error
	ok   int
}

func (t *fetchTally) succeed() { t.ok++ }

func (t *fetchTally) fail(source string, err error) {
	t.errs = append(t.errs, err)
	t.meta.FetchFailures = append(t.meta.FetchFailures, fetchFailure{Source: source, URL: sources.ErrorURL(err), Error: sources.Describe(err)})
}

// allFailed returns nil when at least one fetch succeeded.
func (t *fetchTally) allFailed(what string) error {
	if t.ok > 0 || len(t.errs) == 0 {
		return nil
	}
	return sourceFailure(fmt.Errorf("%s: %w", what, errors.Join(t.errs...)), t.errs...)
}

// sourceFailure wraps err as a rate-limit error (exit 7) when every cause is
// a 429, otherwise as an API error (exit 5).
func sourceFailure(err error, causes ...error) error {
	for _, c := range causes {
		if !sources.IsRateLimit(c) {
			return apiErr(err)
		}
	}
	return rateLimitErr(err)
}

// printAttribution writes each distinct source credit in meta once.
func printAttribution(w io.Writer, meta respMeta) {
	seen := map[string]bool{}
	for _, s := range meta.Sources {
		if s.Attribution != "" && !seen[s.Attribution] {
			seen[s.Attribution] = true
			fmt.Fprintf(w, "%s — %s\n", s.Attribution, s.Link)
		}
	}
}

// lookupParkArg resolves a park argument or returns a usage error that lists
// the valid keys.
func lookupParkArg(arg string) (parks.Park, error) {
	p, ok := parks.Lookup(arg)
	if !ok {
		return parks.Park{}, usageErr(fmt.Errorf("unknown park %q (use one of: %s)", arg, strings.Join(parks.Keys(), ", ")))
	}
	return p, nil
}

// resolveDBPath returns the history database path. SQLite treats '?' and '#'
// in a path as DSN syntax, so such paths are rejected instead of guessed.
func resolveDBPath(path string) (string, error) {
	if path == "" {
		return defaultDBPath("japan-theme-parks-pp-cli"), nil
	}
	if strings.ContainsAny(path, "?#") {
		return "", fmt.Errorf("--db path %q must not contain '?' or '#'", path)
	}
	return path, nil
}

// history is the matching local snapshot history for one park.
type history struct {
	Exists bool
	Span   store.WaitSpan  // every stored row of the park, unfiltered
	Rows   []store.WaitRow // rows that match the query
	Bad    int             // matching rows skipped: unreadable timestamps
}

// loadHistory reads stored snapshots read-only with the filters applied in
// SQL. A missing file is not an error; it returns Exists=false. Warnings go
// to errOut.
func loadHistory(ctx context.Context, errOut io.Writer, flags *rootFlags, dbPath string, q store.WaitQuery) (history, error) {
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		return history{}, nil
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return history{}, fmt.Errorf("opening history database: %w", err)
	}
	defer db.Close()
	if err := db.RejectNewerSchema(); err != nil {
		return history{}, fmt.Errorf("opening history database %s: %w", dbPath, err)
	}
	span, err := db.WaitHistorySpan(ctx, q.ParkID)
	if err != nil {
		return history{}, err
	}
	rows, bad, err := db.QueryWaitRows(ctx, q)
	if err != nil {
		return history{}, err
	}
	if bad > 0 {
		fmt.Fprintf(errOut, "warning: skipped %d stored snapshot rows with unreadable timestamps in %s\n", bad, dbPath)
	}
	staleHistoryHint(errOut, flags, span.Last)
	return history{Exists: true, Span: span, Rows: rows, Bad: bad}, nil
}

func samplesOf(rows []store.WaitRow) []parks.Sample {
	out := make([]parks.Sample, len(rows))
	for i, r := range rows {
		out[i] = parks.Sample{RideID: r.RideID, RideName: r.RideName, Land: r.Land, IsOpen: r.IsOpen, WaitMinutes: r.WaitMinutes, At: r.SourceUpdatedAt}
	}
	return out
}

// boundCtxN gives a command that makes n sequential source fetches a deadline
// of n times --timeout, so paced multi-fetch commands are not cut short. A
// positive --rate-limit adds the time spent waiting between requests, which
// --timeout (a per-request budget) does not cover.
func boundCtxN(parent context.Context, flags *rootFlags, n int) (context.Context, context.CancelFunc) {
	if flags == nil || flags.timeout <= 0 {
		return parent, func() {}
	}
	if n < 1 {
		n = 1
	}
	return context.WithTimeout(parent, flags.timeout*time.Duration(n)+pacingBudget(flags.rateLimit, n))
}

// pacingBudget is the time n requests can spend waiting for a limiter at
// rate requests per second. Zero or auto rates use the per-source defaults
// (1-2 per second), which the per-request timeout already absorbs.
func pacingBudget(rate float64, n int) time.Duration {
	if rate <= 0 || n < 1 {
		return 0
	}
	return time.Duration(float64(n) / rate * float64(time.Second))
}

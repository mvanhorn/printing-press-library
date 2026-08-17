package gaclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/cliutil"
)

func TestYearRange(t *testing.T) {
	cases := []struct {
		from, to     int
		wantF, wantT int
	}{
		{2016, 2026, 2016, 2026}, // normal span
		{2020, 0, 2020, 2020},    // only from → single year
		{0, 2020, 2020, 2020},    // only to → single year
		{2026, 2016, 2016, 2026}, // reversed → swapped
		{2021, 2021, 2021, 2021}, // single year
	}
	for _, c := range cases {
		f, to := yearRange(c.from, c.to)
		if f != c.wantF || to != c.wantT {
			t.Errorf("yearRange(%d,%d) = (%d,%d), want (%d,%d)", c.from, c.to, f, to, c.wantF, c.wantT)
		}
	}
}

func TestDedupKey(t *testing.T) {
	// ECLI wins; then idprovv; then the document coordinates. An item lacking
	// every identifier must still yield a stable, non-empty key so the sweep
	// de-duplicates it instead of repeating it once per year.
	if got := dedupKey(Provvedimento{Ecli: "E", Idprovv: "I"}); got != "E" {
		t.Errorf("ecli should win, got %q", got)
	}
	if got := dedupKey(Provvedimento{Idprovv: "I"}); got != "I" {
		t.Errorf("idprovv fallback, got %q", got)
	}
	noID := Provvedimento{Schema: "tar_na", Nrg: "202102978", NomeFile: "202106765_01.html"}
	if got := dedupKey(noID); got != "tar_na|202102978|202106765_01.html" {
		t.Errorf("coordinate fallback, got %q", got)
	}
	if dedupKey(noID) != dedupKey(noID) {
		t.Errorf("dedupKey not stable for identical items")
	}
}

func TestSweepYearsKeepsResultsWhenAYearFails(t *testing.T) {
	// A transient failure on 2021 must not discard 2020 and 2022: the year is
	// skipped, the union is still returned, and the gap is reported in Warnings.
	fetch := func(y int) (*SearchResult, error) {
		if y == 2021 {
			return nil, errors.New("timeout")
		}
		return &SearchResult{
			Items: []Provvedimento{{Ecli: fmt.Sprintf("ECLI:%d", y)}, {Ecli: "ECLI:DUP"}},
			Total: 10,
		}, nil
	}
	res, err := sweepYears(context.Background(), 2020, 2022, fetch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 2020 + 2022, with the shared ECLI:DUP de-duplicated across years.
	if len(res.Items) != 3 {
		t.Errorf("got %d items, want 3", len(res.Items))
	}
	if res.Total != 20 {
		t.Errorf("got Total %d, want 20 (only the years that succeeded)", res.Total)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "2021") {
		t.Errorf("expected a warning naming 2021, got %v", res.Warnings)
	}
}

func TestSweepYearsErrorsWhenNoYearSucceeds(t *testing.T) {
	// Nothing collected: the caller must see the failure, not an empty result.
	fetch := func(int) (*SearchResult, error) { return nil, errors.New("boom") }
	if _, err := sweepYears(context.Background(), 2020, 2022, fetch); err == nil {
		t.Error("expected an error when every year fails")
	}
}

func TestSweepYearsAbortsOnRateLimit(t *testing.T) {
	// Rate limiting stops the sweep (more years mean more 429s) but keeps what
	// was already collected.
	calls := 0
	fetch := func(y int) (*SearchResult, error) {
		calls++
		if y == 2021 {
			return nil, &cliutil.RateLimitError{URL: "https://x", RetryAfter: time.Second}
		}
		return &SearchResult{Items: []Provvedimento{{Ecli: fmt.Sprintf("ECLI:%d", y)}}, Total: 5}, nil
	}
	res, err := sweepYears(context.Background(), 2020, 2025, fetch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("sweep did not stop at the rate limit: %d fetches", calls)
	}
	if len(res.Items) != 1 {
		t.Errorf("got %d items, want the 2020 result kept", len(res.Items))
	}
	if len(res.Warnings) == 0 {
		t.Error("expected a warning about the interrupted sweep")
	}
}

func TestSweepYearsReportsSkippedYearsWhenItLaterAborts(t *testing.T) {
	// 2021 skipped (transient), 2022 aborts (rate limit): the abort must not
	// hide the fact that 2021 is missing from the range too.
	fetch := func(y int) (*SearchResult, error) {
		switch y {
		case 2021:
			return nil, errors.New("timeout")
		case 2022:
			return nil, &cliutil.RateLimitError{URL: "https://x"}
		}
		return &SearchResult{Items: []Provvedimento{{Ecli: fmt.Sprintf("ECLI:%d", y)}}, Total: 5}, nil
	}
	res, err := sweepYears(context.Background(), 2020, 2025, fetch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joined := strings.Join(res.Warnings, " | ")
	if !strings.Contains(joined, "2021") {
		t.Errorf("the transiently skipped year vanished from the warnings: %q", joined)
	}
	if !strings.Contains(joined, "2022") {
		t.Errorf("the aborting year is not reported: %q", joined)
	}
}

func TestFatalSweepError(t *testing.T) {
	// A transient per-year failure must not abort the sweep (the years already
	// collected would be lost); rate limiting and a cancelled context must.
	live := context.Background()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	cases := []struct {
		name string
		ctx  context.Context
		err  error
		want bool
	}{
		{"network timeout", live, &net.DNSError{Err: "timeout", IsTimeout: true}, false},
		{"generic parse failure", live, errors.New("nessun risultato nella pagina"), false},
		{"wrapped transient", live, fmt.Errorf("anno 2019: %w", errors.New("EOF")), false},
		{"rate limit", live, &cliutil.RateLimitError{URL: "https://x", RetryAfter: time.Second}, true},
		{"wrapped rate limit", live, fmt.Errorf("anno 2019: %w", &cliutil.RateLimitError{URL: "https://x"}), true},
		{"context cancelled", cancelled, errors.New("request failed"), true},
		{"deadline exceeded", live, context.DeadlineExceeded, true},
	}
	for _, c := range cases {
		if got := fatalSweepError(c.ctx, c.err); got != c.want {
			t.Errorf("%s: fatalSweepError = %v, want %v", c.name, got, c.want)
		}
	}
}

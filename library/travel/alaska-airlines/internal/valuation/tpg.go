// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-05-20: value-compare) — TPG monthly-valuations
// scraper. Public marketing page (no auth, no Cloudflare in normal
// conditions). Parses the table row for a given program and extracts
// the cents-per-point float. Probe date: 2026-05-20.

package valuation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// TPGValuationsURL is the canonical source URL for TPG's monthly
// valuations. Surfaced in the comparator's meta envelope so agents can
// re-fetch or audit.
const TPGValuationsURL = "https://thepointsguy.com/loyalty-programs/monthly-valuations/"

// Typed errors surfaced by FetchTPGValuation. The Lookup soft-fallback
// chain branches on these.
var (
	// ErrTPGFetch indicates a network-level failure (DNS, timeout,
	// connection reset, non-2xx without a bot-challenge body).
	ErrTPGFetch = errors.New("tpg fetch failed")
	// ErrTPGParse indicates the page loaded but the program's row or
	// numeric cell could not be located — typically a TPG redesign.
	ErrTPGParse = errors.New("tpg parse failed")
	// ErrTPGBlocked indicates a probable Cloudflare / bot-challenge
	// interstitial.
	ErrTPGBlocked = errors.New("tpg blocked")
)

// tpgClient is the package-level HTTP client. 15 s timeout is more than
// generous for a static marketing page.
var tpgClient = &http.Client{Timeout: 15 * time.Second}

// chromeUA mirrors a current desktop Chrome User-Agent. TPG occasionally
// 403s requests with Go's default UA; sending a real-looking Chrome UA
// avoids the simplest filter.
const chromeUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// cppRegexp pulls the first decimal-or-integer value from a TPG cell.
// TPG annotates some cells with "*" (methodology pending) or "†"
// (footnote). We strip those before parsing.
var cppRegexp = regexp.MustCompile(`(\d+(?:\.\d+)?)`)

// FetchTPGValuation issues one GET against TPG's monthly-valuations page
// and returns the cents-per-point value for the program identified by
// def.TPGRowMatch. Caller is expected to cache the result.
func FetchTPGValuation(ctx context.Context, def ProgramDef) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, TPGValuationsURL, nil)
	if err != nil {
		return 0, fmt.Errorf("%w: build request: %v", ErrTPGFetch, err)
	}
	req.Header.Set("User-Agent", chromeUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := tpgClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrTPGFetch, err)
	}
	defer resp.Body.Close()

	// Cap the body read at 2 MiB. The TPG monthly-valuations page is
	// well under 1 MiB today, but a CDN redirect or a runaway page
	// shouldn't be able to OOM the CLI.
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if readErr != nil {
		return 0, fmt.Errorf("%w: read body: %v", ErrTPGFetch, readErr)
	}

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusServiceUnavailable {
		if looksLikeCloudflareChallenge(body) {
			return 0, fmt.Errorf("%w: status %d cloudflare-shaped body", ErrTPGBlocked, resp.StatusCode)
		}
		return 0, fmt.Errorf("%w: status %d", ErrTPGFetch, resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("%w: status %d", ErrTPGFetch, resp.StatusCode)
	}

	// Cloudflare can also serve a 200 with a "Just a moment..."
	// interstitial.
	if looksLikeCloudflareChallenge(body) {
		return 0, fmt.Errorf("%w: 200 with cloudflare-shaped body", ErrTPGBlocked)
	}

	return parseTPGTable(body, def.TPGRowMatch)
}

// parseTPGTable walks the TPG monthly-valuations HTML and returns the
// cents-per-point value for the row whose first cell case-insensitive-
// contains rowMatch. Exposed for testing on synthetic HTML fixtures.
func parseTPGTable(body []byte, rowMatch string) (float64, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return 0, fmt.Errorf("%w: build doc: %v", ErrTPGParse, err)
	}

	needle := strings.ToLower(strings.TrimSpace(rowMatch))
	if needle == "" {
		return 0, fmt.Errorf("%w: empty row match", ErrTPGParse)
	}

	var cpp float64
	var found bool

	doc.Find("tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		cells := row.Find("td")
		if cells.Length() < 2 {
			return true // continue
		}
		first := strings.ToLower(strings.TrimSpace(cells.Eq(0).Text()))
		if !strings.Contains(first, needle) {
			return true
		}
		value := strings.TrimSpace(cells.Eq(1).Text())
		match := cppRegexp.FindString(value)
		if match == "" {
			// Row matched but no parseable value — keep looking; TPG
			// occasionally has a header row whose first cell repeats
			// the program name in a styled context.
			return true
		}
		var parseErr error
		if _, parseErr = fmt.Sscanf(match, "%f", &cpp); parseErr != nil {
			return true
		}
		found = true
		return false // break
	})

	if !found {
		return 0, fmt.Errorf("%w: no row matching %q with parseable value", ErrTPGParse, rowMatch)
	}
	if cpp <= 0 || cpp > 10 {
		// Sanity guard — published cpp values cluster between 0.5 and
		// 5. Anything outside that range is almost certainly a parse
		// error against a column with a different meaning.
		return 0, fmt.Errorf("%w: extracted cpp %.4f out of plausible range", ErrTPGParse, cpp)
	}
	return cpp, nil
}

// looksLikeCloudflareChallenge inspects a response body for the common
// Cloudflare bot-fight markers.
func looksLikeCloudflareChallenge(body []byte) bool {
	lower := strings.ToLower(string(body))
	for _, marker := range []string{
		"just a moment...",
		"cf-browser-verification",
		"cf-challenge",
		"attention required! | cloudflare",
		"checking your browser before accessing",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

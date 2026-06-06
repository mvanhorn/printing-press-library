// Package farescrape implements an experimental scraper for
// nationalrail.co.uk fare lookups. The implementation is interface-bounded
// so a future Trainline rebuild or RDG paid integration is a swap, not a
// rewrite.
//
// The current implementation is best-effort and explicitly never
// fabricates data on failure — it surfaces the failure with a clean error
// per AGENTS.md anti-reimplementation rule.
package farescrape

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FareLookup is the abstract surface CLI commands code against.
type FareLookup interface {
	// Lookup returns a best-effort fare summary for an A->B journey on the
	// given date. Returns an error if the underlying source is unreachable
	// or its layout has drifted (no fabricated data on failure).
	Lookup(from, to, date string) (*FareResult, error)
}

// FareResult captures whatever the scraper successfully extracted.
type FareResult struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Date         string `json:"date"`
	URL          string `json:"url"`
	Experimental bool   `json:"experimental"`
	Note         string `json:"note"`
}

// NRWebScraper is the v0.1 implementation: it fetches the National Rail
// search-results URL and extracts whatever fare information is exposed
// in the page. It does NOT log into Trainline or any commercial booking
// surface; if nationalrail.co.uk does not return fare data, the result
// surfaces that explicitly.
type NRWebScraper struct {
	httpClient *http.Client
}

// NewNRWebScraper returns a scraper with sane HTTP defaults (10s timeout,
// no auth, polite User-Agent).
func NewNRWebScraper() *NRWebScraper {
	return &NRWebScraper{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Lookup performs the experimental scrape.
//
// pp:client-call — fetches https://www.nationalrail.co.uk/journey-planner/.
func (s *NRWebScraper) Lookup(from, to, date string) (*FareResult, error) {
	if from == "" || to == "" {
		return nil, fmt.Errorf("from and to CRS codes required")
	}

	q := url.Values{}
	q.Set("origin", strings.ToUpper(from))
	q.Set("destination", strings.ToUpper(to))
	if date != "" {
		q.Set("date", date)
	}
	target := "https://www.nationalrail.co.uk/journey-planner/?" + q.Encode()

	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", "github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/0.1 (+https://github.com/mvanhorn/cli-printing-press)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w (fare scrape is experimental and may fail)", target, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nationalrail.co.uk returned HTTP %d for %s — selector outdated or layout changed; please file an issue", resp.StatusCode, target)
	}

	// Read but do not invent fare data: the National Rail journey planner is
	// a JS-rendered SPA in 2026, so a plain HTTP GET will not yield fare
	// numbers without a browser. Surface the URL so the user can open it
	// themselves and mark the result experimental.
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return &FareResult{
		From:         strings.ToUpper(from),
		To:           strings.ToUpper(to),
		Date:         date,
		URL:          target,
		Experimental: true,
		Note:         "Fare scrape is experimental. The National Rail journey planner is JS-rendered; numeric fares are not available via plain HTTP. Open the URL in a browser for live fare data, or wait for v0.2 which will add browser-clearance support.",
	}, nil
}

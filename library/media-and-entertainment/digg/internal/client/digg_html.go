// Hand-written client extensions for di.gg HTML pages that don't have
// JSON-API equivalents (e.g. /ai/1000). Lives alongside the generated
// client.go (which is owned by the Printing Press generator) so future
// regeneration doesn't blow these methods away.
//
// PATCH(library-side): added by U4 of the digg search/roster plan.

package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
)

// roster1000URL is the canonical /ai/1000 page URL. Lives on di.gg, not
// the API host the generated client points at. Defined as a const so
// tests can substitute via FetchRoster1000From.
const roster1000URL = "https://di.gg/ai/1000"

// FetchRoster1000 GETs the /ai/1000 page, hands the HTML to the RSC
// parser, and returns the structured roster. Uses the client's
// configured HTTP client (with the same impersonation, timeout, and
// rate-limit guarantees as JSON API calls) so live runs share one
// connection pool. No retries beyond what the underlying transport does
// — the page is large but stable; retrying a 5xx is more likely to
// hammer Digg than to recover.
func (c *Client) FetchRoster1000(ctx context.Context) ([]diggparse.Roster1000Author, error) {
	return c.FetchRoster1000From(ctx, roster1000URL)
}

// FetchRoster1000From is FetchRoster1000 with a caller-supplied URL.
// Exists so unit tests can point at a local httptest server.
func (c *Client) FetchRoster1000From(ctx context.Context, url string) ([]diggparse.Roster1000Author, error) {
	cctx, cancel := context.WithTimeout(ctx, c.ConfiguredTimeout())
	defer cancel()

	c.limiter.Wait()

	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "digg-pp-cli/0.1.0 (+https://github.com/mvanhorn/printing-press-library)")
	req.Header.Set("Accept", "text/html")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading /ai/1000 body: %w", err)
	}
	authors, err := diggparse.ParseRoster1000(body)
	// Parser returns partial results + error on partial parse; surface both.
	return authors, err
}

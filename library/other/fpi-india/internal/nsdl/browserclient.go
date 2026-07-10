package nsdl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/enetx/surf"

	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/cliutil"
)

// browserLimiter paces the handful of Chrome-fingerprint fetch paths
// (NSDL StaticReports, CDSL) that bypass the generated client's own
// cliutil.AdaptiveLimiter. Shared across calls so bursts (a sync loop
// walking sector fortnights, or a live typed-endpoint read that re-fetches
// through this path) still ramp/halve as one budget instead of one
// per-call limiter that never learns.
var browserLimiter = cliutil.NewAdaptiveLimiterAuto(2.0)

// browserRateLimitError is the typed 429 signal for this package's fetch
// paths, distinguishing "server throttled us" from "no data exists" for
// callers deciding whether to retry or surface an error.
type browserRateLimitError struct {
	URL string
}

func (e *browserRateLimitError) Error() string {
	return fmt.Sprintf("rate limited fetching %s", e.URL)
}

// browserFingerprintHTTPGet fetches a URL through a Chrome-TLS-fingerprint
// client. NSDL's StaticReports family (sector fortnightly archive, trade-wise
// equity/debt) sits behind bot mitigation that rejects Go's stock net/http
// TLS ClientHello with "Request Rejected" even when the User-Agent header
// matches a real browser exactly — curl and a real browser both pass, plain
// Go net/http does not. This is the same class of protection the Printing
// Press's http_transport: browser-chrome spec option exists to work around
// (see internal/generator/templates/client.go.tmpl); scoped here to just
// these two fetch paths rather than switching the whole CLI's transport,
// since every other endpoint (Yearwise, ReportDetail, DefaultAPI_Reports)
// is unaffected and already works over plain HTTP.
func browserFingerprintHTTPGet(ctx context.Context, url string) ([]byte, error) {
	surfClient := surf.NewClient().Builder().
		Impersonate().Chrome().
		Timeout(30 * time.Second).
		Build().Unwrap()
	httpClient := surfClient.Std()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	browserLimiter.Wait()
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		browserLimiter.OnRateLimit()
		return nil, &browserRateLimitError{URL: url}
	}
	browserLimiter.OnSuccess()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// A missing dated snapshot (bad date, not-yet-published, retired path)
	// answers with a real 404/4xx status carrying an HTML error page as the
	// body. Surface that as an error instead of handing the caller an HTML
	// error page to parse as if it were a report.
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetching %s: HTTP %d", url, resp.StatusCode)
	}
	return body, nil
}

// FetchStaticReport fetches a path under NSDL's StaticReports family through
// the Chrome-fingerprint client, joining it with baseURL when path is not
// already absolute.
func FetchStaticReport(ctx context.Context, baseURL, path string) ([]byte, error) {
	url := path
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = strings.TrimRight(baseURL, "/") + path
	}
	return browserFingerprintHTTPGet(ctx, url)
}

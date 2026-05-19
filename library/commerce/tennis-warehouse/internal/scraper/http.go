package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/tennis-warehouse/internal/cliutil"
)

// chromeUA is the User-Agent Tennis Warehouse expects to serve full HTML.
// The site degrades response shape for a generic curl UA, so always send Chrome.
const chromeUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// HTTPClient fetches Tennis Warehouse pages with rate limiting and retries.
type HTTPClient struct {
	BaseURL string
	HTTP    *http.Client
	Limiter *cliutil.AdaptiveLimiter
}

// NewHTTPClient builds a Tennis Warehouse-tuned HTTP client.
func NewHTTPClient(ratePerSec float64) *HTTPClient {
	if ratePerSec <= 0 {
		ratePerSec = 1.0
	}
	return &HTTPClient{
		BaseURL: "https://www.tennis-warehouse.com",
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Limiter: cliutil.NewAdaptiveLimiter(ratePerSec),
	}
}

// Fetch returns the body of a GET request. Retries once on transient errors and
// returns a typed *cliutil.RateLimitError when 429s exhaust retries.
func (c *HTTPClient) Fetch(ctx context.Context, path string) (string, error) {
	url := path
	if !strings.HasPrefix(path, "http") {
		url = c.BaseURL + path
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		c.Limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", chromeUA)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		if resp.StatusCode == 429 {
			c.Limiter.OnRateLimit()
			resp.Body.Close()
			lastErr = &cliutil.RateLimitError{
				URL: url,
			}
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
			continue
		}
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return "", fmt.Errorf("GET %s: HTTP %d: %s", url, resp.StatusCode, snippet(string(body)))
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		c.Limiter.OnSuccess()
		return string(body), nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("GET %s: exhausted retries", url)
}

func snippet(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

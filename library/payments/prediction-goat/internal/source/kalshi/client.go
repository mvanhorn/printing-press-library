package kalshi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/cliutil"
)

const (
	DefaultBaseURL = "https://api.elections.kalshi.com/trade-api/v2"
	Source         = "kalshi"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	limiter    *cliutil.AdaptiveLimiter
}

func New() *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		limiter: cliutil.NewAdaptiveLimiter(5.0),
	}
}

func (c *Client) Get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	body, status, err := c.get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	if status == http.StatusTooManyRequests {
		c.rateLimited()
		if err := sleepContext(ctx, time.Second); err != nil {
			return nil, err
		}
		body, status, err = c.get(ctx, path, params)
		if err != nil {
			return nil, err
		}
		if status == http.StatusTooManyRequests {
			c.rateLimited()
			return nil, &cliutil.RateLimitError{
				URL:        Source,
				RetryAfter: time.Second,
				Body:       string(snippet(body, 200)),
			}
		}
	}
	if status >= 400 {
		return nil, fmt.Errorf("kalshi GET %s: HTTP %d: %s", path, status, snippet(body, 200))
	}
	c.succeeded()
	return body, nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, int, error) {
	baseURL := DefaultBaseURL
	if c != nil && strings.TrimSpace(c.BaseURL) != "" {
		baseURL = strings.TrimRight(c.BaseURL, "/")
	}
	u := baseURL + path
	if encoded := params.Encode(); encoded != "" {
		u += "?" + encoded
	}

	if c != nil && c.limiter != nil {
		c.limiter.Wait()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("kalshi GET %s: build request: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "prediction-goat-pp-cli/1.0")

	httpClient := http.DefaultClient
	if c != nil && c.HTTPClient != nil {
		httpClient = c.HTTPClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("kalshi GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("kalshi GET %s: read response: %w", path, err)
	}
	return body, resp.StatusCode, nil
}

// GetMarket fetches the detail payload for a single market ticker. Unlike
// the /markets list endpoint, /markets/{ticker} includes price fields
// (yes_ask_dollars, no_ask_dollars, last_price_dollars), which is the
// backfill path the sync uses to enrich high-volume active markets after
// the bulk list pass.
func (c *Client) GetMarket(ctx context.Context, ticker string) ([]byte, error) {
	if strings.TrimSpace(ticker) == "" {
		return nil, fmt.Errorf("kalshi GetMarket: empty ticker")
	}
	body, err := c.Get(ctx, "/markets/"+ticker, nil)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (c *Client) rateLimited() {
	if c != nil && c.limiter != nil {
		c.limiter.OnRateLimit()
	}
}

func (c *Client) succeeded() {
	if c != nil && c.limiter != nil {
		c.limiter.OnSuccess()
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func snippet(body []byte, limit int) []byte {
	if len(body) <= limit {
		return body
	}
	return body[:limit]
}

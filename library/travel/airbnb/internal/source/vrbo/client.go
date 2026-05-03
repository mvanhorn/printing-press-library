package vrbo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/cliutil"
)

const (
	baseURL = "https://www.vrbo.com"
	ua      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
)

var defaultClient = &Client{
	http:    &http.Client{Timeout: 30 * time.Second},
	limiter: cliutil.NewAdaptiveLimiter(1.0 / 3.0),
}

type Client struct {
	http       *http.Client
	limiter    *cliutil.AdaptiveLimiter
	mu         sync.Mutex
	lastWarmup time.Time
	detailOp   string
}

func init() {
	jar, _ := cookiejar.New(nil)
	defaultClient.http.Jar = jar
}

func Warmup(ctx context.Context) error { return defaultClient.Warmup(ctx) }
func Search(ctx context.Context, params SearchParams) ([]Property, *Pagination, error) {
	return defaultClient.Search(ctx, params)
}
func Get(ctx context.Context, propertyID string, params GetParams) (*Property, error) {
	return defaultClient.Get(ctx, propertyID, params)
}

func (c *Client) Warmup(ctx context.Context) error {
	c.mu.Lock()
	if time.Since(c.lastWarmup) < 25*time.Minute {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	if _, err := c.do(ctx, "GET", baseURL+"/", nil, baseURL+"/", nil); err != nil {
		return err
	}
	time.Sleep(1500 * time.Millisecond)
	c.mu.Lock()
	c.lastWarmup = time.Now()
	c.mu.Unlock()
	return nil
}

func (c *Client) Search(ctx context.Context, params SearchParams) ([]Property, *Pagination, error) {
	return nil, nil, ErrDisabled
}

func (c *Client) Get(ctx context.Context, propertyID string, params GetParams) (*Property, error) {
	return nil, ErrDisabled
}

func (c *Client) do(ctx context.Context, method, target string, body io.Reader, referer string, extra map[string]string) ([]byte, error) {
	const retries = 3
	for attempt := 0; attempt <= retries; attempt++ {
		c.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, method, target, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Origin", baseURL)
		req.Header.Set("Referer", referer)
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Dest", "empty")
		for k, v := range extra {
			req.Header.Set(k, v)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == 429 {
			c.limiter.OnRateLimit()
			if attempt == retries {
				return nil, &cliutil.RateLimitError{URL: target, RetryAfter: cliutil.RetryAfter(resp), Body: string(data)}
			}
			time.Sleep(cliutil.RetryAfter(resp))
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("%s %s returned HTTP %d: %s", method, target, resp.StatusCode, truncate(string(data)))
		}
		c.limiter.OnSuccess()
		return data, nil
	}
	return nil, fmt.Errorf("request failed")
}

func (c *Client) searchHTML(ctx context.Context, params SearchParams) ([]Property, error) {
	u := searchURL(params)
	data, err := c.do(ctx, "GET", u, nil, u, nil)
	if err != nil {
		return nil, err
	}
	props, err := propertiesFromSearchHTML(data)
	if err != nil {
		if warmErr := c.Warmup(ctx); warmErr != nil {
			return nil, warmErr
		}
		data, retryErr := c.do(ctx, "GET", u, nil, u, nil)
		if retryErr != nil {
			return nil, retryErr
		}
		return propertiesFromSearchHTML(data)
	}
	return props, nil
}

func (c *Client) getHTML(ctx context.Context, id string, params GetParams) (*Property, error) {
	u := detailURL(id, params)
	data, err := c.do(ctx, "GET", u, nil, u, nil)
	if err != nil {
		return nil, err
	}
	prop, err := propertyFromDetailHTML(data, id, u)
	if err != nil {
		return nil, err
	}
	return prop, nil
}

func searchURL(params SearchParams) string {
	u, _ := url.Parse(baseURL + "/search")
	q := u.Query()
	if params.Location != "" {
		q.Set("destination", params.Location)
	}
	if params.Checkin != "" {
		q.Set("d1", params.Checkin)
		q.Set("startDate", params.Checkin)
	}
	if params.Checkout != "" {
		q.Set("d2", params.Checkout)
		q.Set("endDate", params.Checkout)
	}
	if adults := defaultInt(params.Adults, 1); adults > 0 {
		q.Set("adults", fmt.Sprintf("%d", adults))
	}
	q.Set("flexibility", "0_DAY")
	u.RawQuery = q.Encode()
	return u.String()
}

func detailURL(id string, params GetParams) string {
	id = normalizePropertyID(id)
	u, _ := url.Parse(baseURL + "/h" + url.PathEscape(id))
	q := u.Query()
	if params.Checkin != "" {
		q.Set("d1", params.Checkin)
		q.Set("startDate", params.Checkin)
	}
	if params.Checkout != "" {
		q.Set("d2", params.Checkout)
		q.Set("endDate", params.Checkout)
	}
	if adults := defaultInt(params.Adults, 1); adults > 0 {
		q.Set("adults", fmt.Sprintf("%d", adults))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

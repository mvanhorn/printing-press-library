package toyota

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/internal/cliutil"
	"golang.org/x/net/html"
)

const maxBody = 2 << 20
const maxRequests = 14

type Client struct {
	http     *http.Client
	origin   string
	limiter  *cliutil.AdaptiveLimiter
	start    time.Time
	mu       sync.Mutex
	requests int
	bytes    int64
}

type sourceTransport struct {
	c    *Client
	base http.RoundTripper
}

func (t sourceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c := t.c
	c.mu.Lock()
	if c.requests >= maxRequests {
		c.mu.Unlock()
		return nil, &SourceError{"Toyota request budget exhausted; use a narrower request"}
	}
	c.requests++
	c.mu.Unlock()
	if err := c.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if remaining, resetAt, ok := cliutil.ParseRateLimitHeaders(resp.Header); ok {
		c.limiter.ObserveHeaders(remaining, resetAt)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		retry := cliutil.RetryAfter(resp)
		closeErr := resp.Body.Close()
		return nil, &cliutil.RateLimitError{URL: req.URL.Scheme + "://" + req.URL.Host + req.URL.Path, RetryAfter: retry, Cause: closeErr}
	}
	c.limiter.OnSuccess()
	return resp, nil
}

// NewClient preserves the root request-rate contract: negative means adaptive
// auto pacing, zero disables pacing, and a positive value is a hard ceiling.
func NewClient(rateLimit float64) *Client {
	c := newClient(Origin, http.DefaultTransport)
	if rateLimit < 0 {
		c.limiter = cliutil.NewAdaptiveLimiterAuto(2)
	} else {
		c.limiter = cliutil.NewAdaptiveLimiter(rateLimit)
	}
	return c
}

func newClient(origin string, transport http.RoundTripper) *Client {
	jar, _ := cookiejar.New(nil)
	c := &Client{origin: origin, limiter: cliutil.NewAdaptiveLimiter(2), start: time.Now()}
	c.http = &http.Client{Jar: jar, Transport: sourceTransport{c, transport},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 6 {
				return &SourceError{"Toyota redirect limit exceeded"}
			}
			u, _ := url.Parse(origin)
			if req.URL.Scheme != u.Scheme || req.URL.Host != u.Host {
				return &SourceError{"Toyota redirected outside its first-party origin; request stopped"}
			}
			return nil
		}}
	return c
}

func (c *Client) Meta() Metadata {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Metadata{Source: "live", FetchedAt: time.Now().UTC().Format(time.RFC3339), Requests: c.requests, ResponseBytes: c.bytes, ElapsedMS: time.Since(c.start).Milliseconds(), MaxRequests: maxRequests}
}

func (c *Client) request(ctx context.Context, path string, form url.Values) ([]byte, *html.Node, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, "", err
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, nil, "", &InputError{"invalid first-party source path"}
	}
	method := http.MethodGet
	var body io.Reader
	if form != nil {
		method = http.MethodPost
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.origin+path, body)
	if err != nil {
		return nil, nil, "", err
	}
	req.Header.Set("User-Agent", "toyota-rentacar-pp-cli/0.1 (public read-only planning)")
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Referer", c.origin+path)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, "", fmt.Errorf("Toyota request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, resp.Request.URL.String(), &SourceError{fmt.Sprintf("Toyota HTTP %d at %s; retry or open %s", resp.StatusCode, resp.Request.URL.Path, Origin+BookingPath)}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, nil, "", fmt.Errorf("read Toyota response: %w", err)
	}
	if len(data) > maxBody {
		return nil, nil, "", &SourceError{"Toyota response exceeds 2MiB; source layout may have changed"}
	}
	c.mu.Lock()
	c.bytes += int64(len(data))
	c.mu.Unlock()
	doc, err := parseHTML(data)
	if err != nil {
		return nil, nil, "", err
	}
	final := resp.Request.URL.String()
	if err := sourceProblem(doc, final); err != nil {
		return nil, nil, final, err
	}
	return data, doc, final, nil
}

func (c *Client) SearchShops(ctx context.Context, keyword string, limit int) (ShopsResult, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || len([]rune(keyword)) > 80 {
		return ShopsResult{}, &InputError{"--keyword must contain 1–80 characters"}
	}
	if limit < 1 || limit > 10 {
		return ShopsResult{}, &InputError{"--limit must be between 1 and 10 (Toyota returns at most ten nearby shops)"}
	}
	path := BookingPath + "?" + url.Values{"keyword": {keyword}, "shopMode": {"0"}}.Encode()
	data, _, _, err := c.request(ctx, path, nil)
	if err != nil {
		return ShopsResult{}, err
	}
	shops, sourceContext, err := ParseShops(data)
	if err != nil {
		return ShopsResult{}, err
	}
	if len(shops) > 0 && !strings.Contains(strings.ToLower(sourceContext), strings.ToLower(keyword)) {
		return ShopsResult{}, &SourceError{"Toyota returned a different search context " + sourceContext + "; no unrelated shops returned"}
	}
	count := len(shops)
	if len(shops) > limit {
		shops = shops[:limit]
	}
	result := ShopsResult{Meta: c.Meta(), Query: keyword, SourceContext: sourceContext, Shops: shops, SourceCount: count, Truncated: count > len(shops), Note: "Operating hours and closure notes are not dated vehicle availability."}
	if count == 0 {
		result.Note = "Toyota returned no matching shops for this keyword; try a station or airport name."
	}
	return result, nil
}

func (c *Client) shop(ctx context.Context, id, pathRoot string, returnMode bool) (Shop, *html.Node, string, error) {
	r, e, err := ParseShopID(id)
	if err != nil {
		return Shop{}, nil, "", err
	}
	mode := "0"
	if returnMode {
		mode = "1"
	}
	path := pathRoot + "?" + url.Values{"shopMode": {mode}, "rShop": {r}, "eShop": {e}}.Encode()
	data, doc, final, err := c.request(ctx, path, nil)
	if err != nil {
		return Shop{}, nil, "", err
	}
	shops, _, err := ParseShops(data)
	if err != nil {
		return Shop{}, nil, "", err
	}
	for _, s := range shops {
		if s.ID == id {
			return s, doc, final, nil
		}
	}
	return Shop{}, nil, final, &NotFoundError{"Toyota did not return requested shop " + id + "; resolve a valid ID with shops search"}
}

func (c *Client) GetShop(ctx context.Context, id string) (Shop, error) {
	s, _, _, err := c.shop(ctx, id, BookingPath, false)
	return s, err
}

func pathOf(u string) string { x, _ := url.Parse(u); return x.RequestURI() }

func fillShopPair(f url.Values, doc *html.Node, pickup, dropoff Shop) error {
	r, e, _ := ParseShopID(pickup.ID)
	rr, ee, _ := ParseShopID(dropoff.ID)
	for id, v := range map[string]string{"txtDepShop": pickup.Name, "hdDepShop": pickup.Name, "txtRetShop": dropoff.Name, "hdRetShop": dropoff.Name, "txtHdnDepRCode": r, "txtHdnDepECode": e, "txtHdnRetRCode": rr, "txtHdnRetECode": ee} {
		if err := setID(f, doc, id, v); err != nil {
			return err
		}
	}
	return nil
}

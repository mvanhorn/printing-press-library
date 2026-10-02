// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package ferry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/internal/cliutil"
)

const maxBody = 2 << 20
const maxRequests = 10

type Client struct {
	http     *http.Client
	limiter  *cliutil.AdaptiveLimiter
	started  time.Time
	requests int
	bytes    int
	sources  []string
}

func New(rate float64) *Client {
	if rate <= 0 || rate > 2 {
		rate = 2
	}
	jar, _ := cookiejar.New(nil)
	c := &Client{limiter: cliutil.NewAdaptiveLimiter(rate), started: time.Now()}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = 15 * time.Second
	c.http = &http.Client{Jar: jar, Timeout: 20 * time.Second, Transport: &boundedTransport{client: c, next: tr}}
	c.http.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return fmt.Errorf("source redirect limit exceeded")
		}
		if !allowedURL(req.URL, req.Method) {
			return fmt.Errorf("source redirected outside the read-only planning boundary")
		}
		return nil
	}
	return c
}

type boundedTransport struct {
	client *Client
	next   http.RoundTripper
}

func (b *boundedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c := b.client
	if !allowedURL(req.URL, req.Method) {
		return nil, fmt.Errorf("request outside the read-only first-party planning boundary")
	}
	if c.requests >= maxRequests {
		return nil, fmt.Errorf("source request budget exhausted")
	}
	if err := c.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	c.requests++
	return b.next.RoundTrip(req)
}
func allowedURL(u *url.URL, method string) bool {
	if u.Scheme != "https" || u.User != nil || u.Port() != "" || u.RawQuery != "" && !strings.HasPrefix(u.Path, "/route/inc/cal-config/") {
		return false
	}
	switch u.Hostname() {
	case "www.ferry-sunflower.co.jp":
		if method != "GET" {
			return false
		}
		if u.Path == "/en/reservation/" {
			return true
		}
		for _, r := range Registry {
			for _, s := range []string{"time", "fee", "cabin", "boarding"} {
				if u.Path == "/en/route/"+r.ID+"/"+s+"/" {
					return true
				}
			}
		}
		return u.Path == "/route/inc/cal-config/cal-beppu.js" || u.Path == "/route/inc/cal-config/cal-kyusyu.js"
	case "www.sunflower.co.jp":
		return method == "GET" && u.Path == "/stipulate/passenger/"
	case "booking.ferry-sunflower.co.jp":
		if method == "POST" {
			return u.Path == "/web/yoyaku/Reserve0000/Reserve" || u.Path == "/web/yoyaku/Reserve1030/MoveNext"
		}
		return method == "GET" && (u.Path == "/web/yoyaku/Reserve0000/IndexEnglish" || u.Path == "/web/yoyaku/Reserve1030" || u.Path == "/web/yoyaku/Reserve1020")
	}
	return false
}
func (c *Client) fetch(ctx context.Context, address string, form url.Values) ([]byte, error) {
	method := http.MethodGet
	var body io.Reader
	if form != nil {
		method = http.MethodPost
		body = strings.NewReader(form.Encode())
	}
	u, err := url.Parse(address)
	if err != nil || !allowedURL(u, method) {
		return nil, fmt.Errorf("invalid first-party source request")
	}
	req, err := http.NewRequestWithContext(ctx, method, address, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sunflower-ferry-pp-cli/0.1 (read-only planning)")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", BookingBase)
		req.Header.Set("Referer", BookingURL)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("source request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: address, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("official source returned HTTP %d; no planning data inferred", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxBody {
		return nil, fmt.Errorf("official source body exceeded 2 MiB")
	}
	c.bytes += len(b)
	c.limiter.OnSuccess()
	found := false
	for _, s := range c.sources {
		if s == address {
			found = true
		}
	}
	if !found {
		c.sources = append(c.sources, address)
	}
	return b, nil
}
func (c *Client) Meta(language string) Metadata {
	return Metadata{ObservedAt: time.Now().In(JST).Format(time.RFC3339), SourceURLs: append([]string{}, c.sources...), SourceLanguage: language, RequestCount: c.requests, ResponseBytes: c.bytes, ElapsedMS: time.Since(c.started).Milliseconds()}
}

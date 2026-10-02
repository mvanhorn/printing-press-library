// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package yamato

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/yamato/internal/cliutil"
	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
	"golang.org/x/text/encoding/japanese"
)

const Main = "https://www.kuronekoyamato.co.jp"
const Date = "https://date.kuronekoyamato.co.jp/date/"
const SameDayPDF = Main + "/ytc/en/send/services/baggage-branch-list/pdf/same-day_delivery.pdf"
const CounterList = Main + "/ytc/en/send/services/airport/list.html"
const ObservedAt = "2026-10-02T01:32:00+09:00"

type Source struct {
	URL       string `json:"url"`
	FetchedAt string `json:"fetched_at"`
	Bytes     int    `json:"bytes"`
	SHA256    string `json:"sha256"`
}
type Meta struct {
	Source        string         `json:"source"`
	Timezone      string         `json:"timezone"`
	Requests      int            `json:"upstream_requests"`
	ElapsedMS     int64          `json:"elapsed_ms"`
	Sources       []Source       `json:"sources"`
	FetchFailures []FetchFailure `json:"fetch_failures,omitempty"`
}
type FetchFailure struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}
type Client struct {
	HTTP     *http.Client
	Sources  []Source
	Started  time.Time
	Requests int
	limiter  *cliutil.AdaptiveLimiter
	Failures []FetchFailure
}

func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	// The airport calculator requires the public session established by its form.
	// Keep those cookies only in memory for this command invocation.
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	c := &Client{Started: time.Now(), Sources: []Source{}, limiter: cliutil.NewAdaptiveLimiter(2)}
	c.HTTP = &http.Client{Timeout: timeout, Jar: jar, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("Yamato redirect limit exceeded")
		}
		if !AllowedURL(r.URL.String()) {
			return fmt.Errorf("Yamato redirected outside the supported first-party origins")
		}
		c.Requests++
		return nil
	}}
	return c
}
func AllowedURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && (u.Host == "www.kuronekoyamato.co.jp" || u.Host == "date.kuronekoyamato.co.jp")
}
func (c *Client) Meta() Meta {
	return Meta{"live", "Asia/Tokyo", c.Requests, time.Since(c.Started).Milliseconds(), c.Sources, c.Failures}
}
func (c *Client) Fetch(ctx context.Context, raw string, form url.Values, max int64) ([]byte, error) {
	if !AllowedURL(raw) {
		return nil, fmt.Errorf("unsupported Yamato source URL")
	}
	if e := c.limiter.Wait(ctx); e != nil {
		return nil, e
	}
	method := http.MethodGet
	var reader io.Reader
	if form != nil {
		method = http.MethodPost
		reader = strings.NewReader(form.Encode())
	}
	req, e := http.NewRequestWithContext(ctx, method, raw, reader)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "yamato-pp-cli/0.1 (+read-only public luggage planning)")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	c.Requests++
	res, e := c.HTTP.Do(req)
	if e != nil {
		return nil, fmt.Errorf("Yamato source request failed: %w; retry or open %s", e, raw)
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, max+1))
	if e != nil {
		return nil, fmt.Errorf("read Yamato response: %w", e)
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("Yamato response exceeds %d-byte bound", max)
	}
	if res.StatusCode == 429 {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: raw, RetryAfter: cliutil.RetryAfter(res), Body: "Yamato public source throttled; retry later"}
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Yamato returned HTTP %d for %s; open the source or retry later", res.StatusCode, raw)
	}
	c.limiter.OnSuccess()
	hash := fmt.Sprintf("%x", sha256.Sum256(b))
	c.Sources = append(c.Sources, Source{raw, time.Now().UTC().Format(time.RFC3339), len(b), hash})
	if strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "windows-31j") || strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "shift_jis") {
		b, e = japanese.ShiftJIS.NewDecoder().Bytes(b)
		if e != nil {
			return nil, fmt.Errorf("decode Yamato Windows-31J response: %w", e)
		}
	}
	return b, nil
}
func Parse(b []byte) (*html.Node, error) { return html.Parse(bytes.NewReader(b)) }
func Attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}
func Text(n *html.Node) string {
	var chunks []string
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.ElementNode && (x.Data == "script" || x.Data == "style") {
			return
		}
		if x.Type == html.TextNode {
			chunks = append(chunks, x.Data)
		}
		for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(strings.Join(chunks, " ")), " ")
}
func Nodes(n *html.Node, tag string) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.ElementNode && x.Data == tag {
			out = append(out, x)
		}
		for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return out
}
func Rows(table *html.Node) [][]string {
	var out [][]string
	for _, tr := range Nodes(table, "tr") {
		var row []string
		for ch := tr.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type == html.ElementNode && (ch.Data == "td" || ch.Data == "th") {
				row = append(row, Text(ch))
			}
		}
		if len(row) > 0 {
			out = append(out, row)
		}
	}
	return out
}

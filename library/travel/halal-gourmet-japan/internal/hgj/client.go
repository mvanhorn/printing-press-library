// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hgj

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/cliutil"
)

const maxBody = 8 << 20

var prefectures = strings.Fields("Hokkaido Aomori Iwate Miyagi Akita Yamagata Fukushima Ibaraki Tochigi Gunma Saitama Chiba Tokyo Kanagawa Niigata Toyama Ishikawa Fukui Yamanashi Nagano Gifu Shizuoka Aichi Shiga Kyoto Osaka Hyogo Nara Wakayama Mie Tottori Shimane Okayama Hiroshima Yamaguchi Tokushima Kagawa Ehime Kochi Fukuoka Saga Nagasaki Kumamoto Oita Miyazaki Kagoshima Okinawa")

type HTTPError struct {
	Status int
	URL    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Halal Gourmet Japan returned HTTP %d for %s; inspect the canonical source page", e.Status, e.URL)
}

type Client struct {
	BaseURL string
	HTTP    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func NewClient(base string, timeout time.Duration, rate float64) *Client {
	if base == "" {
		base = Origin
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if rate < 0 {
		rate = 1
	}
	if rate > 2 {
		rate = 2
	}
	u, _ := url.Parse(base)
	return &Client{BaseURL: strings.TrimRight(base, "/"), HTTP: &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many source redirects")
		}
		if u == nil || req.URL.Host != u.Host || req.URL.Scheme != u.Scheme {
			return fmt.Errorf("source redirected outside its configured origin")
		}
		return nil
	}}, limiter: cliutil.NewAdaptiveLimiter(rate)}
}

type SearchOptions struct {
	Kind           string
	Query          string
	Prefecture     string
	Genre          string
	Features       []string
	PlaceType      string
	PrayerFeatures []string
	Limit          int
}

func (o SearchOptions) Values() (url.Values, error) {
	if o.Kind != Restaurant && o.Kind != Prayer {
		return nil, fmt.Errorf("kind must be restaurant or prayer")
	}
	if o.Query == "" && o.Prefecture == "" {
		return nil, fmt.Errorf("provide --prefecture or --query to bound source discovery")
	}
	if len([]rune(o.Query)) > 200 || len([]rune(o.Genre)) > 80 {
		return nil, fmt.Errorf("--query must be at most 200 characters and --genre at most 80")
	}
	if o.Limit < 1 || o.Limit > 50 {
		return nil, fmt.Errorf("--limit must be between 1 and 50")
	}
	v := url.Values{}
	v.Set("category", o.Kind)
	if o.Query != "" {
		v.Set("q", o.Query)
	}
	if o.Prefecture != "" {
		found := ""
		for _, p := range prefectures {
			if strings.EqualFold(p, o.Prefecture) {
				found = p
				break
			}
		}
		if found == "" {
			return nil, fmt.Errorf("unknown --prefecture %q; use a Japanese prefecture name such as Tokyo, Kyoto or Osaka", o.Prefecture)
		}
		v.Set("prefecture", found)
	}
	if o.Kind == Restaurant {
		if o.Genre != "" {
			v.Set("genre", o.Genre)
		}
		for _, f := range o.Features {
			k, err := NormalizeCondition(f)
			if err != nil {
				return nil, err
			}
			if !contains(foodKeys, k) {
				return nil, fmt.Errorf("--feature %q is a prayer-place condition; use prayer search --prayer-feature", f)
			}
			v.Add("feature", k)
		}
	} else {
		if o.PlaceType != "" && o.PlaceType != "spaces" && o.PlaceType != "mosques" {
			return nil, fmt.Errorf("--place-type must be spaces or mosques")
		}
		if o.PlaceType != "" {
			v.Set("placeType", o.PlaceType)
		}
		for _, f := range o.PrayerFeatures {
			k, err := NormalizeCondition(f)
			if err != nil {
				return nil, err
			}
			if !contains(prayerKeys, k) {
				return nil, fmt.Errorf("--prayer-feature %q must be wudu, wifi, hotWater or qibla", f)
			}
			v.Add("prayerFeature", k)
		}
	}
	return v, nil
}
func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
func (c *Client) fetch(ctx context.Context, path string, v url.Values) ([]byte, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, err
	}
	u.RawQuery = v.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "Halal-Gourmet-Japan-CLI/1.0 (+https://github.com/mvanhorn/printing-press-library)")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("source request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: u.String(), RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{resp.StatusCode, u.String()}
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		return nil, fmt.Errorf("source format changed: expected text/html, received %q", resp.Header.Get("Content-Type"))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("source body read: %w", err)
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("source response exceeded the 8 MiB cap; narrow --prefecture or --query")
	}
	c.limiter.OnSuccess()
	return body, nil
}
func (c *Client) Search(ctx context.Context, o SearchOptions) (SearchResult, error) {
	v, err := o.Values()
	if err != nil {
		return SearchResult{}, err
	}
	body, err := c.fetch(ctx, "/search", v)
	if err != nil {
		return SearchResult{}, err
	}
	return ParseSearch(body, o.Kind, Origin+"/search?"+v.Encode(), observedNow(), o.Limit)
}
func (c *Client) Detail(ctx context.Context, kind, id string) (Place, error) {
	canonical, err := CanonicalURL(kind, id)
	if err != nil {
		return Place{}, err
	}
	path := strings.TrimPrefix(canonical, Origin)
	body, err := c.fetch(ctx, path, url.Values{})
	if err != nil {
		return Place{}, err
	}
	return ParseDetail(body, kind, id, observedNow())
}

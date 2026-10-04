// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package ticket

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/cliutil"
)

const MaxBody = 512 * 1024

type HTTPError struct {
	Status int
	URL    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Toretabi GET %s returned HTTP%d", e.URL, e.Status)
}

type Client struct {
	HTTP    *http.Client
	limiter *cliutil.AdaptiveLimiter
	origin  string
}

func NewClient(rate ...float64) *Client {
	rps := 2.0
	if len(rate) > 0 && rate[0] > 0 && rate[0] < rps {
		rps = rate[0]
	}
	return &Client{HTTP: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("Toretabi redirect limit exceeded")
		}
		if req.URL.Scheme != "https" || req.URL.Host != "www.toretabi.jp" {
			return errors.New("Toretabi redirected outside its public origin")
		}
		return nil
	}}, limiter: cliutil.NewAdaptiveLimiter(rps), origin: Origin}
}
func (c *Client) fetch(ctx context.Context, path string) ([]byte, string, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, "", err
	}
	raw := c.origin + path
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if e != nil {
		return nil, "", e
	}
	req.Header.Set("User-Agent", "toretabi-pp-cli/0.0.0-dev")
	req.Header.Set("Accept", "text/html")
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return nil, "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, "", &cliutil.RateLimitError{URL: raw, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", &HTTPError{Status: resp.StatusCode, URL: raw}
	}
	c.limiter.OnSuccess()
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		return nil, "", fmt.Errorf("Toretabi response is not HTML")
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if e != nil {
		return nil, "", e
	}
	if len(b) > MaxBody {
		return nil, "", fmt.Errorf("Toretabi page exceeded %d-byte bound", MaxBody)
	}
	return b, time.Now().UTC().Format(time.RFC3339Nano), nil
}
func (c *Client) Get(ctx context.Context, id string) (Ticket, error) {
	if !ValidID(id) {
		return Ticket{}, fmt.Errorf("ticket ID must be a source ID such as tokai_043")
	}
	b, at, e := c.fetch(ctx, "/ticket/"+id+".html")
	if e != nil {
		return Ticket{}, e
	}
	return ParseDetail(b, id, at)
}

type ListOptions struct {
	Area     string
	Type     string
	Query    string
	MaxPages int
	Limit    int
}

func (c *Client) List(ctx context.Context, opt ListOptions) (Listing, error) {
	out := Listing{Tickets: []Summary{}, Areas: []CatalogEntry{}, Types: []CatalogEntry{}, Coverage: Coverage{Routes: []string{}, MaxPages: opt.MaxPages, Note: "Coverage describes this native-filtered bounded scan, not all Japan tickets."}}
	if opt.MaxPages < 1 || opt.MaxPages > 5 || opt.Limit < 1 || opt.Limit > 50 {
		return out, fmt.Errorf("--max-pages must be 1..5 and --limit 1..50")
	}
	q := url.Values{}
	if opt.Area != "" {
		n, e := strconv.Atoi(opt.Area)
		if e != nil || n < 1 || n > 11 {
			return out, fmt.Errorf("--area must be source code 1..11")
		}
		q.Set("area", opt.Area)
	}
	if opt.Type != "" {
		n, e := strconv.Atoi(opt.Type)
		if e != nil || n < 1 || n > 4 {
			return out, fmt.Errorf("--ticket-type must be source code 1..4")
		}
		q.Set("class", opt.Type)
	}
	seen := map[string]bool{}
	for p := 1; p <= opt.MaxPages; p++ {
		path := "/ticket/"
		if p > 1 {
			path = "/ticket/page/" + strconv.Itoa(p) + "/"
		}
		if len(q) > 0 {
			path += "?" + q.Encode()
		}
		b, at, e := c.fetch(ctx, path)
		if e != nil {
			return out, e
		}
		page, e := ParseListing(b, at)
		if e != nil {
			return out, e
		}
		out.ObservedAt = at
		if p == 1 {
			out.Areas = page.Areas
			out.Types = page.Types
		}
		out.Coverage.Routes = append(out.Coverage.Routes, c.origin+path)
		out.Coverage.ScannedPages++
		for _, t := range page.Tickets {
			if seen[t.ID] {
				continue
			}
			seen[t.ID] = true
			out.Coverage.ScannedTickets++
			if opt.Query != "" && !strings.Contains(strings.ToLower(t.NameJA+" "+strings.Join(t.TagsJA, " ")), strings.ToLower(opt.Query)) {
				continue
			}
			out.Coverage.MatchedTickets++
			if len(out.Tickets) < opt.Limit {
				out.Tickets = append(out.Tickets, t)
			}
		}
		// A native next-page link is required; record its query with the current native filters.
		nextPath := "/ticket/page/" + strconv.Itoa(p+1) + "/"
		hasNext := strings.Contains(string(b), `href="`+Origin+nextPath) || strings.Contains(string(b), `href="`+nextPath)
		if !hasNext {
			out.Coverage.Continuation = nil
			out.Coverage.Complete = true
			break
		}
		if len(q) > 0 {
			nextPath += "?" + q.Encode()
		}
		out.Coverage.Continuation = strptr(c.origin + nextPath)
	}
	out.Coverage.Returned = len(out.Tickets)
	if out.Coverage.MatchedTickets > len(out.Tickets) {
		out.Coverage.Note += " Output was capped by --limit."
	}
	if !out.Coverage.Complete {
		out.Coverage.Note += " More source pages remain; widen --max-pages deliberately."
	}
	if len(out.Tickets) == 0 {
		out.Coverage.Note += " Zero matches apply only to scanned tickets."
	}
	return out, nil
}

package duckduckgo

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/searchbackend"
)

type Backend struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func init() {
	searchbackend.Register("ddg", func() searchbackend.Backend {
		return &Backend{http: &http.Client{Timeout: 20 * time.Second}, limiter: cliutil.NewAdaptiveLimiter(0.5)}
	})
	searchbackend.Register("duckduckgo", func() searchbackend.Backend {
		return &Backend{http: &http.Client{Timeout: 20 * time.Second}, limiter: cliutil.NewAdaptiveLimiter(0.5)}
	})
}

func (b *Backend) Name() string { return "ddg" }

func (b *Backend) Search(ctx context.Context, query string, opts searchbackend.SearchOpts) ([]searchbackend.Result, error) {
	u := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	var doc *goquery.Document
	for attempt := 0; attempt <= 3; attempt++ {
		b.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := b.http.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 429 {
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			b.limiter.OnRateLimit()
			if attempt == 3 {
				return nil, &cliutil.RateLimitError{URL: u, RetryAfter: cliutil.RetryAfter(resp), Body: string(data)}
			}
			time.Sleep(cliutil.RetryAfter(resp))
			continue
		}
		if resp.StatusCode >= 400 {
			resp.Body.Close()
			return nil, &url.Error{Op: "GET", URL: u, Err: errors.New(resp.Status)}
		}
		doc, err = goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		b.limiter.OnSuccess()
		break
	}
	if doc == nil {
		return nil, searchbackend.ErrUnsupported
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	var out []searchbackend.Result
	doc.Find(".result").EachWithBreak(func(i int, s *goquery.Selection) bool {
		a := s.Find(".result__a").First()
		title := strings.TrimSpace(a.Text())
		href, _ := a.Attr("href")
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())
		if title != "" && href != "" {
			if parsed, err := url.Parse(href); err == nil {
				if uddg := parsed.Query().Get("uddg"); uddg != "" {
					href = uddg
				}
			}
			out = append(out, searchbackend.Result{Title: title, URL: href, Snippet: snippet, Domain: domainOf(href), Score: 1 - float64(len(out))*0.05})
		}
		return len(out) < limit
	})
	return out, nil
}

func (b *Backend) ImageSearch(context.Context, string, searchbackend.SearchOpts) ([]searchbackend.Result, error) {
	return nil, searchbackend.ErrUnsupported
}

func domainOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

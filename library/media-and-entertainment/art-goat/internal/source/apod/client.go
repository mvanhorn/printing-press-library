// Copyright 2026 justinwfu. Licensed under Apache-2.0.

// Package apod implements the art-goat Source for NASA's Astronomy
// Picture of the Day. Uses DEMO_KEY by default (rate-limited ~30/hr);
// users can supply a free key via ART_GOAT_API_KEY or NASA_API_KEY env.
package apod

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source"
)

const (
	baseURL        = "https://api.nasa.gov/planetary/apod"
	userAgent      = "art-goat-pp-cli (contemplative art practice)"
	curatedDefault = 500
	firstAPODDate  = "1995-06-16"
)

func init() {
	source.Register(&Client{
		http: &http.Client{Timeout: 30 * time.Second},
		// NASA APOD: DEMO_KEY = ~30 req/hr (~1 every 2 minutes); a
		// personal key buys ~1000/hr. Start conservative at 0.5/sec so a
		// burst sync with DEMO_KEY doesn't immediately trip the 429
		// ceiling. AdaptiveLimiter ramps up on consecutive successes.
		limiter: cliutil.NewAdaptiveLimiter(0.5),
	})
}

type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func (c *Client) Name() string {
	return "apod"
}

func (c *Client) Description() string {
	return "NASA Astronomy Picture of the Day — daily image + curator essay since 1995-06-16"
}

func (c *Client) AuthRequired() bool {
	// DEMO_KEY works without signup, so we report false. Users may still
	// upgrade to a free key to escape the rate limit; the auth wizard
	// surfaces that as optional, not required.
	return false
}

func (c *Client) apiKey() string {
	for _, name := range []string{"ART_GOAT_API_KEY", "NASA_API_KEY"} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v
		}
	}
	return "DEMO_KEY"
}

// apodEntry mirrors NASA APOD's JSON response shape.
type apodEntry struct {
	Date           string `json:"date"`
	Title          string `json:"title"`
	Explanation    string `json:"explanation"`
	URL            string `json:"url"`
	HDURL          string `json:"hdurl"`
	MediaType      string `json:"media_type"`
	ServiceVersion string `json:"service_version"`
	Copyright      string `json:"copyright"`
	ThumbnailURL   string `json:"thumbnail_url"`
}

// Sync pulls APOD entries from the most recent N days back. Curated
// default = last 500 days (~1.4 years). Full mode walks the full archive
// from 1995-06-16, but most users won't want that — APOD's daily archive
// is small enough that ~500 entries gives plenty of variety for a daily
// practice without saturating the local store.
func (c *Client) Sync(ctx context.Context, opts source.SyncOpts) ([]source.Work, error) {
	limit := opts.Limit
	if limit <= 0 {
		if opts.Full {
			limit = 11000 // ~30 years of entries since 1995
		} else {
			limit = curatedDefault
		}
	}

	// NASA APOD's count= parameter returns N random entries, which is the
	// fastest way to bulk-populate the curated default. For Full mode we
	// walk by date range in 100-day chunks (NASA caps a date range at
	// roughly 100 days per request).
	if !opts.Full {
		return c.syncRandom(ctx, limit)
	}
	return c.syncRange(ctx, limit)
}

// syncRandom asks NASA APOD for N random entries in one call. NASA caps
// count at 100; we loop in batches.
func (c *Client) syncRandom(ctx context.Context, limit int) ([]source.Work, error) {
	out := make([]source.Work, 0, limit)
	for len(out) < limit {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}

		remaining := limit - len(out)
		batch := remaining
		if batch > 100 {
			batch = 100
		}

		q := url.Values{}
		q.Set("api_key", c.apiKey())
		q.Set("count", fmt.Sprintf("%d", batch))
		q.Set("thumbs", "true")

		entries, err := c.fetchEntries(ctx, q)
		if err != nil {
			return out, err
		}
		if len(entries) == 0 {
			break
		}
		for _, e := range entries {
			w := apodEntryToWork(e)
			if w.ID == "" {
				continue
			}
			out = append(out, w)
			if len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}

// syncRange walks the date range from firstAPODDate to today in 100-day
// chunks. Returns all entries up to limit.
func (c *Client) syncRange(ctx context.Context, limit int) ([]source.Work, error) {
	start, err := time.Parse("2006-01-02", firstAPODDate)
	if err != nil {
		return nil, fmt.Errorf("parse first APOD date: %w", err)
	}
	today := time.Now().UTC()

	out := make([]source.Work, 0, limit)
	chunkStart := start
	for chunkStart.Before(today) && len(out) < limit {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}

		chunkEnd := chunkStart.AddDate(0, 0, 90)
		if chunkEnd.After(today) {
			chunkEnd = today
		}

		q := url.Values{}
		q.Set("api_key", c.apiKey())
		q.Set("start_date", chunkStart.Format("2006-01-02"))
		q.Set("end_date", chunkEnd.Format("2006-01-02"))
		q.Set("thumbs", "true")

		entries, err := c.fetchEntries(ctx, q)
		if err != nil {
			return out, err
		}
		for _, e := range entries {
			w := apodEntryToWork(e)
			if w.ID == "" {
				continue
			}
			out = append(out, w)
			if len(out) >= limit {
				return out, nil
			}
		}
		chunkStart = chunkEnd.AddDate(0, 0, 1)
	}
	return out, nil
}

// fetchEntries hits NASA APOD with the prepared query and decodes the
// response. The endpoint returns a single object for a single-day query
// and a JSON array for count/range queries. NASA APOD enforces a rate
// limit (~30/hr with DEMO_KEY, ~1000/hr with a personal key) and returns
// 429 plus a Retry-After header when exceeded — fetchEntries honors that
// header with bounded retries before giving up.
func (c *Client) fetchEntries(ctx context.Context, q url.Values) ([]apodEntry, error) {
	const maxAttempts = 3
	var body []byte
	var resp *http.Response
	for attempt := 0; attempt < maxAttempts; attempt++ {
		c.limiter.Wait()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+q.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("apod build request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err = c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("apod request: %w", err)
		}
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("apod read body: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			c.limiter.OnRateLimit()
			if attempt == maxAttempts-1 {
				return nil, fmt.Errorf("apod rate-limited (HTTP %d) after %d attempts: %s",
					resp.StatusCode, maxAttempts, cliutil.TruncateBytes(body, 200))
			}
			wait := cliutil.ParseRetryAfter(resp.Header.Get("Retry-After"))
			if wait <= 0 {
				wait = time.Duration(attempt+1) * 2 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		if resp.StatusCode == http.StatusOK {
			c.limiter.OnSuccess()
		}
		break
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apod returned %d: %s", resp.StatusCode, cliutil.TruncateBytes(body, 200))
	}

	// Probe first non-whitespace character to choose array vs object decode.
	trimmed := strings.TrimLeft(string(body), " \t\n\r")
	if strings.HasPrefix(trimmed, "[") {
		var arr []apodEntry
		if err := json.Unmarshal(body, &arr); err != nil {
			return nil, fmt.Errorf("apod decode array: %w", err)
		}
		return arr, nil
	}
	var single apodEntry
	if err := json.Unmarshal(body, &single); err != nil {
		return nil, fmt.Errorf("apod decode object: %w", err)
	}
	return []apodEntry{single}, nil
}

func apodEntryToWork(e apodEntry) source.Work {
	if e.Date == "" {
		return source.Work{}
	}
	t, _ := time.Parse("2006-01-02", e.Date)
	year := t.Year()
	w := source.Work{
		ID:               "apod:" + e.Date,
		Source:           "apod",
		SourceID:         e.Date,
		Title:            strings.TrimSpace(e.Title),
		Creator:          strings.TrimSpace(e.Copyright),
		CreatorCanonical: strings.ToLower(strings.TrimSpace(e.Copyright)),
		DateText:         e.Date,
		DateStart:        year,
		DateEnd:          year,
		Medium:           "Photograph",
		Classification:   "Astronomy",
		CultureRegion:    "Cosmos",
		Description:      strings.TrimSpace(e.Explanation),
		SourceURL:        fmt.Sprintf("https://apod.nasa.gov/apod/ap%s.html", strings.ReplaceAll(e.Date[2:], "-", "")),
		License:          "Public Domain (NASA)",
		SyncedAt:         time.Now().UTC(),
	}
	if e.MediaType == "image" {
		w.ImageURL = e.URL
		if e.HDURL != "" {
			w.ImageURL = e.HDURL
		}
		w.ThumbnailURL = e.URL
	} else if e.MediaType == "video" {
		// Videos: keep the page URL as ImageURL so `sit` can still emit something.
		w.ImageURL = e.URL
		w.ThumbnailURL = e.ThumbnailURL
		w.Medium = "Video"
	}
	if e.Copyright == "" {
		w.Creator = "NASA"
		w.CreatorCanonical = "nasa"
	}
	rawBytes, _ := json.Marshal(e)
	w.RawJSON = string(rawBytes)
	return w
}

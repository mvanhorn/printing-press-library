// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

// Package cleveland implements the art-goat Source for the Cleveland
// Museum of Art's Open Access API (openaccess-api.clevelandart.org).
// Anonymous; no key required. We restrict to CC0-licensed works that have
// a web image so every synced record is both displayable and freely
// reusable in a contemplative practice.
package cleveland

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source"
)

const (
	baseURL        = "https://openaccess-api.clevelandart.org/api/artworks/"
	userAgent      = "art-goat-pp-cli (contemplative art practice)"
	curatedDefault = 1000
	pageSize       = 100
)

func init() {
	source.Register(&Client{
		http: &http.Client{Timeout: 30 * time.Second},
		// Cleveland's Open Access API has no documented per-IP rate limit
		// but the team asks clients to be reasonable. 2 req/sec keeps a
		// full curated sync well under load; AdaptiveLimiter ramps up on
		// success and halves if the server starts returning 429/503.
		limiter: cliutil.NewAdaptiveLimiter(2.0),
	})
}

type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func (c *Client) Name() string {
	return "cleveland"
}

func (c *Client) Description() string {
	return "Cleveland Museum of Art Open Access — CC0-licensed works with images, anonymous"
}

func (c *Client) AuthRequired() bool {
	return false
}

// clevelandCreator is one entry in an artwork's creators array. The
// description field is the human-readable display string Cleveland
// recommends (e.g., "Katsushika Hokusai (Japanese, 1760-1849)").
type clevelandCreator struct {
	Description string `json:"description"`
}

// clevelandImageVariant covers the {url} shape Cleveland nests under
// images.web and images.print. Other fields exist (width/height/filename)
// but we only need the URL for art-goat's unified store.
type clevelandImageVariant struct {
	URL string `json:"url"`
}

type clevelandImages struct {
	Web   clevelandImageVariant `json:"web"`
	Print clevelandImageVariant `json:"print"`
}

// clevelandArtwork mirrors the subset of fields art-goat reads. Cleveland
// returns creation_date_earliest / creation_date_latest as integers
// (negative for BCE); we keep them as int and default to 0 when unknown.
type clevelandArtwork struct {
	ID                   int                `json:"id"`
	AccessionNumber      string             `json:"accession_number"`
	Title                string             `json:"title"`
	Creators             []clevelandCreator `json:"creators"`
	CreationDate         string             `json:"creation_date"`
	CreationDateEarliest int                `json:"creation_date_earliest"`
	CreationDateLatest   int                `json:"creation_date_latest"`
	Technique            string             `json:"technique"`
	Type                 string             `json:"type"`
	Department           string             `json:"department"`
	Culture              []string           `json:"culture"`
	Description          string             `json:"description"`
	Images               clevelandImages    `json:"images"`
	URL                  string             `json:"url"`
}

type clevelandResponse struct {
	Data []clevelandArtwork `json:"data"`
}

// Sync paginates the Cleveland Open Access API in limit/skip batches.
// Curated default = 1000 works, restricted with has_image=1 and cc0=1 so
// every record is displayable and CC0. Full mode (opts.Full=true) walks
// further but still honors opts.Limit when set.
func (c *Client) Sync(ctx context.Context, opts source.SyncOpts) ([]source.Work, error) {
	limit := opts.Limit
	if limit <= 0 {
		if opts.Full {
			limit = 30000 // safe ceiling; Cleveland's CC0+image subset is ~30k
		} else {
			limit = curatedDefault
		}
	}

	out := make([]source.Work, 0, limit)
	skip := 0
	for len(out) < limit {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}

		batch := pageSize
		if remaining := limit - len(out); remaining < batch {
			batch = remaining
		}

		q := url.Values{}
		q.Set("limit", strconv.Itoa(batch))
		q.Set("skip", strconv.Itoa(skip))
		q.Set("has_image", "1")
		q.Set("cc0", "1")

		artworks, err := c.fetchPage(ctx, q)
		if err != nil {
			return out, err
		}
		if len(artworks) == 0 {
			break
		}
		for _, a := range artworks {
			w := clevelandArtworkToWork(a)
			if w.ID == "" {
				continue
			}
			out = append(out, w)
			if len(out) >= limit {
				return out, nil
			}
		}
		skip += len(artworks)
		// Short page = upstream exhausted; stop instead of looping forever.
		if len(artworks) < batch {
			break
		}
	}
	return out, nil
}

// fetchPage hits Cleveland with the prepared query and decodes the
// response. Cleveland's Open Access API has no documented rate-limit
// header set but returns 429/503 under burst load; fetchPage honors
// Retry-After with bounded retries before giving up.
func (c *Client) fetchPage(ctx context.Context, q url.Values) ([]clevelandArtwork, error) {
	const maxAttempts = 3
	var body []byte
	var resp *http.Response
	for attempt := 0; attempt < maxAttempts; attempt++ {
		c.limiter.Wait()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+q.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("cleveland build request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err = c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("cleveland request: %w", err)
		}
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("cleveland read body: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			c.limiter.OnRateLimit()
			if attempt == maxAttempts-1 {
				return nil, fmt.Errorf("cleveland rate-limited (HTTP %d) after %d attempts: %s",
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
		return nil, fmt.Errorf("cleveland returned %d: %s", resp.StatusCode, cliutil.TruncateBytes(body, 200))
	}

	var decoded clevelandResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("cleveland decode: %w", err)
	}
	return decoded.Data, nil
}

func clevelandArtworkToWork(a clevelandArtwork) source.Work {
	if a.ID == 0 {
		return source.Work{}
	}
	sid := strconv.Itoa(a.ID)
	creator := ""
	if len(a.Creators) > 0 {
		creator = strings.TrimSpace(a.Creators[0].Description)
	}
	culture := ""
	if len(a.Culture) > 0 {
		culture = strings.TrimSpace(a.Culture[0])
	}
	thumb := a.Images.Print.URL
	if thumb == "" {
		thumb = a.Images.Web.URL
	}
	w := source.Work{
		ID:               "cleveland:" + sid,
		Source:           "cleveland",
		SourceID:         sid,
		Title:            strings.TrimSpace(a.Title),
		Creator:          creator,
		CreatorCanonical: strings.ToLower(creator),
		DateText:         strings.TrimSpace(a.CreationDate),
		DateStart:        a.CreationDateEarliest,
		DateEnd:          a.CreationDateLatest,
		Medium:           strings.TrimSpace(a.Technique),
		Classification:   strings.TrimSpace(a.Type),
		Period:           strings.TrimSpace(a.Department),
		CultureRegion:    culture,
		Description:      strings.TrimSpace(a.Description),
		ImageURL:         a.Images.Web.URL,
		ThumbnailURL:     thumb,
		License:          "CC0",
		SourceURL:        a.URL,
		SyncedAt:         time.Now().UTC(),
	}
	rawBytes, _ := json.Marshal(a)
	w.RawJSON = string(rawBytes)
	return w
}

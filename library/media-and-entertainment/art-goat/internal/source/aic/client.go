// Copyright 2026 justinwfu. Licensed under Apache-2.0.

// Package aic implements the art-goat Source for the Art Institute of
// Chicago's public API (api.artic.edu). Anonymous; no key required.
// Honors AIC's recommended User-Agent attribution requirement.
package aic

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
	baseURL        = "https://api.artic.edu/api/v1"
	userAgent      = "art-goat-pp-cli (contemplative art practice)"
	curatedDefault = 5000
)

func init() {
	source.Register(&Client{
		http: &http.Client{Timeout: 30 * time.Second},
		// AIC API has no documented per-IP rate limit but the team asks
		// clients to be reasonable. 2 req/sec keeps a full curated sync
		// well under load; AdaptiveLimiter ramps up on success and halves
		// if the server starts returning 429/503.
		limiter: cliutil.NewAdaptiveLimiter(2.0),
	})
}

type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func (c *Client) Name() string {
	return "aic"
}

func (c *Client) Description() string {
	return "Art Institute of Chicago — ~132k artworks, anonymous, CC0/CC-By"
}

func (c *Client) AuthRequired() bool {
	return false
}

// aicArtwork is the subset of the AIC artworks endpoint response art-goat
// uses. Fields are listed via the `fields=` query param to keep responses
// small.
type aicArtwork struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	ArtistDisplay    string `json:"artist_display"`
	ArtistTitle      string `json:"artist_title"`
	DateDisplay      string `json:"date_display"`
	DateStart        *int   `json:"date_start"`
	DateEnd          *int   `json:"date_end"`
	MediumDisplay    string `json:"medium_display"`
	Classification   string `json:"classification_title"`
	StyleTitle       string `json:"style_title"`
	PlaceOfOrigin    string `json:"place_of_origin"`
	Description      string `json:"description"`
	ShortDescription string `json:"short_description"`
	ImageID          string `json:"image_id"`
	IsPublicDomain   bool   `json:"is_public_domain"`
	HasNotBeenViewed bool   `json:"has_not_been_viewed_much"`
	IsBoosted        bool   `json:"is_boosted"`
	Thumbnail        *struct {
		AltText string `json:"alt_text"`
	} `json:"thumbnail"`
}

type aicResponse struct {
	Pagination struct {
		Total       int    `json:"total"`
		Limit       int    `json:"limit"`
		Offset      int    `json:"offset"`
		TotalPages  int    `json:"total_pages"`
		CurrentPage int    `json:"current_page"`
		NextURL     string `json:"next_url"`
	} `json:"pagination"`
	Data   []aicArtwork `json:"data"`
	Config struct {
		IIIFURL    string `json:"iiif_url"`
		WebsiteURL string `json:"website_url"`
	} `json:"config"`
}

// Sync pulls boosted public-domain artworks (curated highlights) from
// AIC. With opts.Full=true it pulls everything with a public-domain image
// up to opts.Limit (or unlimited if Limit==0). AIC's pagination caps at
// 100 per page; we walk pages until the limit is reached.
func (c *Client) Sync(ctx context.Context, opts source.SyncOpts) ([]source.Work, error) {
	limit := opts.Limit
	if limit <= 0 {
		if opts.Full {
			limit = 50000
		} else {
			limit = curatedDefault
		}
	}

	fields := []string{
		"id", "title", "artist_display", "artist_title",
		"date_display", "date_start", "date_end",
		"medium_display", "classification_title", "style_title",
		"place_of_origin", "description", "short_description",
		"image_id", "is_public_domain", "is_boosted", "thumbnail",
	}

	// Curated default uses is_boosted=true filter (the highlights);
	// Full mode walks all artworks with images.
	q := url.Values{}
	q.Set("fields", strings.Join(fields, ","))
	q.Set("limit", "100")

	out := make([]source.Work, 0, limit)
	iiifBase := ""

	for page := 1; len(out) < limit; page++ {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}

		q.Set("page", strconv.Itoa(page))

		endpoint := baseURL + "/artworks?" + q.Encode()
		body, statusCode, err := c.fetchWithBackoff(ctx, endpoint, page)
		if err != nil {
			return out, err
		}
		if statusCode != http.StatusOK {
			return out, fmt.Errorf("aic page %d returned %d: %s", page, statusCode, cliutil.TruncateBytes(body, 200))
		}

		var decoded aicResponse
		if err := json.Unmarshal(body, &decoded); err != nil {
			return out, fmt.Errorf("aic decode page %d: %w", page, err)
		}
		if iiifBase == "" {
			iiifBase = decoded.Config.IIIFURL
		}

		for _, a := range decoded.Data {
			if a.ImageID == "" {
				continue // Skip works without images
			}
			if !opts.Full && !a.IsBoosted && !a.IsPublicDomain {
				continue // Curated default keeps boosted + public-domain works
			}
			w := source.Work{
				ID:               fmt.Sprintf("aic:%d", a.ID),
				Source:           "aic",
				SourceID:         strconv.Itoa(a.ID),
				Title:            strings.TrimSpace(a.Title),
				Creator:          strings.TrimSpace(a.ArtistTitle),
				CreatorCanonical: canonicalize(a.ArtistTitle),
				DateText:         a.DateDisplay,
				Medium:           a.MediumDisplay,
				Classification:   a.Classification,
				Period:           a.StyleTitle,
				CultureRegion:    placeToRegion(a.PlaceOfOrigin),
				Description:      pickDescription(a.Description, a.ShortDescription),
				SourceURL:        fmt.Sprintf("https://www.artic.edu/artworks/%d", a.ID),
				SyncedAt:         time.Now().UTC(),
			}
			if a.DateStart != nil {
				w.DateStart = *a.DateStart
			}
			if a.DateEnd != nil {
				w.DateEnd = *a.DateEnd
			}
			if a.IsPublicDomain {
				w.License = "Public Domain (CC0)"
			} else {
				w.License = "AIC Terms (CC-By for descriptions)"
			}
			if iiifBase != "" {
				w.ImageURL = fmt.Sprintf("%s/%s/full/843,/0/default.jpg", iiifBase, a.ImageID)
				w.ThumbnailURL = fmt.Sprintf("%s/%s/full/200,/0/default.jpg", iiifBase, a.ImageID)
			}
			rawBytes, _ := json.Marshal(a)
			w.RawJSON = string(rawBytes)
			out = append(out, w)
			if len(out) >= limit {
				break
			}
		}

		if page >= decoded.Pagination.TotalPages {
			break
		}
		if len(decoded.Data) == 0 {
			break
		}
	}

	return out, nil
}

// pickDescription strips HTML and prefers the long description when both
// present. AIC descriptions contain inline <p> markup; we keep the prose
// but drop the tags so the contemplative spine renders cleanly.
func pickDescription(long, short string) string {
	chosen := long
	if strings.TrimSpace(chosen) == "" {
		chosen = short
	}
	if strings.TrimSpace(chosen) == "" {
		return ""
	}
	return stripHTML(chosen)
}

// stripHTML removes simple HTML tags without pulling in golang.org/x/net/html.
// AIC descriptions use <p>, <em>, <i>, <br>, <a href> only.
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	// Collapse multiple whitespace
	out := b.String()
	out = strings.ReplaceAll(out, " ", " ")
	out = strings.Join(strings.Fields(out), " ")
	return out
}

// canonicalize normalizes an artist name for cross-source match.
// Lowercases, trims whitespace, removes common honorifics. Not a full
// Getty ULAN lookup — that's v2 work.
func canonicalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, p := range []string{"sir ", "dame ", "mr. ", "mrs. ", "ms. "} {
		s = strings.TrimPrefix(s, p)
	}
	return s
}

// placeToRegion maps AIC's place_of_origin (a country/region string) to
// the unified culture_region taxonomy art-goat uses. Conservative — when
// in doubt, returns the raw value.
func placeToRegion(place string) string {
	if place == "" {
		return ""
	}
	lower := strings.ToLower(place)
	switch {
	case strings.Contains(lower, "japan"):
		return "Japan"
	case strings.Contains(lower, "china"):
		return "China"
	case strings.Contains(lower, "korea"):
		return "Korea"
	case strings.Contains(lower, "india"):
		return "India"
	case strings.Contains(lower, "egypt"):
		return "Egypt"
	case strings.Contains(lower, "mexico"), strings.Contains(lower, "peru"), strings.Contains(lower, "guatemala"):
		return "Mesoamerica"
	case strings.Contains(lower, "united states"), strings.Contains(lower, "america"):
		return "North America"
	case strings.Contains(lower, "france"), strings.Contains(lower, "italy"), strings.Contains(lower, "spain"),
		strings.Contains(lower, "germany"), strings.Contains(lower, "netherlands"), strings.Contains(lower, "belgium"),
		strings.Contains(lower, "england"), strings.Contains(lower, "united kingdom"), strings.Contains(lower, "europe"):
		return "Europe"
	}
	return place
}

// fetchWithBackoff hits the AIC API at endpoint and retries on 429 or
// 503 using the Retry-After header when present, else exponential
// backoff capped at three attempts. AIC's published guidance asks
// clients to honor Retry-After and reduce request rate when throttled.
func (c *Client) fetchWithBackoff(ctx context.Context, endpoint string, page int) ([]byte, int, error) {
	const maxAttempts = 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		c.limiter.Wait()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("aic build request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("AIC-User-Agent", userAgent)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("aic request page %d: %w", page, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, 0, fmt.Errorf("aic read page %d: %w", page, readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			c.limiter.OnRateLimit()
			if attempt == maxAttempts-1 {
				return nil, resp.StatusCode, fmt.Errorf("aic rate-limited (HTTP %d) page %d after %d attempts: %s",
					resp.StatusCode, page, maxAttempts, cliutil.TruncateBytes(body, 200))
			}
			wait := cliutil.ParseRetryAfter(resp.Header.Get("Retry-After"))
			if wait <= 0 {
				wait = time.Duration(attempt+1) * 2 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, resp.StatusCode, ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		if resp.StatusCode == http.StatusOK {
			c.limiter.OnSuccess()
		}

		return body, resp.StatusCode, nil
	}
	return nil, 0, fmt.Errorf("aic page %d: exhausted retries", page)
}

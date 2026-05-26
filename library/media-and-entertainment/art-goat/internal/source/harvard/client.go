// Copyright 2026 justinwfu. Licensed under Apache-2.0.

// Package harvard implements the art-goat Source for the Harvard Art
// Museums collection API. Requires a free user-supplied API key; signup
// at https://docs.google.com/forms/d/1Fe1H4nOhFkrLpaeBpLAnSrIMYvcAxnYWm0IU9a6IkFA/viewform.
package harvard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source"
)

const (
	baseURL        = "https://api.harvardartmuseums.org/object"
	userAgent      = "art-goat-pp-cli (contemplative art practice)"
	curatedDefault = 2000
	pageSize       = 100
	signupURL      = "https://docs.google.com/forms/d/1Fe1H4nOhFkrLpaeBpLAnSrIMYvcAxnYWm0IU9a6IkFA/viewform"
)

func init() {
	source.Register(&Client{
		http: &http.Client{Timeout: 30 * time.Second},
		// Harvard caps at 2,500 requests/day per key — tighter than Rijks's
		// 10k/day budget. 1 req/sec keeps a curated sync (~20 page calls at
		// size=100) well under the daily ceiling and leaves headroom for a
		// repeat sync the same day. AdaptiveLimiter halves on 429/503.
		limiter: cliutil.NewAdaptiveLimiter(1.0),
	})
}

type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func (c *Client) Name() string {
	return "harvard"
}

func (c *Client) Description() string {
	return "Harvard Art Museums — encyclopedic Western, Asian, Islamic & South Asian collection, requires free API key"
}

func (c *Client) AuthRequired() bool {
	return true
}

func (c *Client) apiKey() string {
	for _, name := range []string{"ART_GOAT_HARVARD_KEY", "HARVARD_API_KEY"} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v
		}
	}
	return ""
}

// harvardPerson mirrors an entry in the people[] array. We prefer the
// entry whose role is "Artist"; collaborations and "After Rubens"
// attributions would otherwise produce noisy Creator values.
type harvardPerson struct {
	PersonID    int    `json:"personid"`
	Name        string `json:"name"`
	DisplayName string `json:"displayname"`
	Role        string `json:"role"`
	DisplayDate string `json:"displaydate"`
}

// harvardObject is the subset of /object response fields the unified Work
// cares about. We keep the full record in RawJSON for forward use.
type harvardObject struct {
	ObjectID             int             `json:"objectid"`
	ObjectNumber         string          `json:"objectnumber"`
	Title                string          `json:"title"`
	Dated                string          `json:"dated"`
	DateBegin            int             `json:"datebegin"`
	DateEnd              int             `json:"dateend"`
	Classification       string          `json:"classification"`
	Century              string          `json:"century"`
	Period               string          `json:"period"`
	Culture              string          `json:"culture"`
	Medium               string          `json:"medium"`
	Technique            string          `json:"technique"`
	PrimaryImageURL      string          `json:"primaryimageurl"`
	BaseImageURL         string          `json:"baseimageurl"`
	People               []harvardPerson `json:"people"`
	URL                  string          `json:"url"`
	Copyright            string          `json:"copyright"`
	AccessLevel          int             `json:"accesslevel"`
	ImagePermissionLevel int             `json:"imagepermissionlevel"`
}

type harvardInfo struct {
	TotalRecords         int    `json:"totalrecords"`
	TotalRecordsPerQuery int    `json:"totalrecordsperquery"`
	Pages                int    `json:"pages"`
	Page                 int    `json:"page"`
	Next                 string `json:"next"`
}

type harvardResponse struct {
	Info    harvardInfo     `json:"info"`
	Records []harvardObject `json:"records"`
}

// Sync walks pages of the Harvard /object endpoint filtered to records
// with images and public accesslevel. Curated default = 2000 works
// (~20 page calls). Full mode walks until the upstream stops returning
// results or the caller-supplied opts.Limit is reached.
func (c *Client) Sync(ctx context.Context, opts source.SyncOpts) ([]source.Work, error) {
	key := c.apiKey()
	if key == "" {
		return nil, errors.New("harvard requires an API key; set HARVARD_API_KEY or ART_GOAT_HARVARD_KEY (free signup at " + signupURL + ")")
	}

	limit := opts.Limit
	if limit <= 0 {
		if opts.Full {
			limit = 50000
		} else {
			limit = curatedDefault
		}
	}

	out := make([]source.Work, 0, limit)

	for page := 1; len(out) < limit; page++ {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}

		q := url.Values{}
		q.Set("apikey", key)
		q.Set("hasimage", "1")
		q.Set("accesslevel", "1")
		q.Set("size", strconv.Itoa(pageSize))
		q.Set("page", strconv.Itoa(page))

		endpoint := baseURL + "?" + q.Encode()
		body, statusCode, err := c.fetchWithBackoff(ctx, endpoint, page)
		if err != nil {
			return out, err
		}
		if statusCode != http.StatusOK {
			return out, fmt.Errorf("harvard page %d returned %d: %s", page, statusCode, cliutil.TruncateBytes(body, 200))
		}

		var decoded harvardResponse
		if err := json.Unmarshal(body, &decoded); err != nil {
			return out, fmt.Errorf("harvard decode page %d: %w", page, err)
		}
		if len(decoded.Records) == 0 {
			break
		}

		for _, r := range decoded.Records {
			if r.ObjectID == 0 {
				continue
			}
			if r.PrimaryImageURL == "" {
				continue
			}
			// imagepermissionlevel: 0=display permitted (full),
			// 1=thumbnail-only display permitted, 2=no image display.
			// Skip 2 entirely. Skip 1 too when baseimageurl is empty —
			// we can't construct the 200px IIIF derivative without it,
			// and surfacing primaryimageurl for a level-1 object would
			// violate Harvard's documented terms.
			if r.ImagePermissionLevel >= 2 {
				continue
			}
			if r.ImagePermissionLevel == 1 && strings.TrimSpace(r.BaseImageURL) == "" {
				continue
			}
			out = append(out, harvardObjectToWork(r))
			if len(out) >= limit {
				break
			}
		}

		// Stop when the API signals no more pages.
		if decoded.Info.Next == "" {
			break
		}
		if decoded.Info.Pages > 0 && page >= decoded.Info.Pages {
			break
		}
	}

	return out, nil
}

// harvardObjectToWork maps one /object record into the unified Work shape.
// Description is intentionally left empty: Harvard's /object endpoint does
// not return a curator essay field. The curator-written text lives at
// /object/:id/contextualtext, which would require one extra request per
// record (~100× the page-call cost) and blow through the 2,500/day cap
// inside a single curated sync. Revisit when the daily ceiling lifts.
func harvardObjectToWork(r harvardObject) source.Work {
	creator := pickCreator(r.People)
	period := strings.TrimSpace(r.Period)
	if period == "" {
		period = strings.TrimSpace(r.Century)
	}
	medium := strings.TrimSpace(r.Medium)
	if medium == "" {
		medium = strings.TrimSpace(r.Technique)
	}

	license := "Public domain (Harvard Art Museums)"
	if cp := strings.TrimSpace(r.Copyright); cp != "" {
		license = cp
	}

	// For level-1 (thumbnail-only) objects, cap the surfaced ImageURL to
	// the 200px IIIF derivative. Harvard's documented permission levels
	// forbid displaying the full primaryimageurl for level-1 records; the
	// Sync filter above already drops level-1 records without a
	// baseimageurl, so thumbnailURL(r) is guaranteed non-empty here.
	imageURL := r.PrimaryImageURL
	if r.ImagePermissionLevel >= 1 {
		imageURL = thumbnailURL(r)
	}

	w := source.Work{
		ID:               "harvard:" + strconv.Itoa(r.ObjectID),
		Source:           "harvard",
		SourceID:         strconv.Itoa(r.ObjectID),
		Title:            strings.TrimSpace(r.Title),
		Creator:          creator,
		CreatorCanonical: strings.ToLower(creator),
		DateText:         strings.TrimSpace(r.Dated),
		DateStart:        r.DateBegin,
		DateEnd:          r.DateEnd,
		Medium:           medium,
		Classification:   strings.TrimSpace(r.Classification),
		Period:           period,
		CultureRegion:    cultureToRegion(r.Culture),
		ImageURL:         imageURL,
		ThumbnailURL:     thumbnailURL(r),
		License:          license,
		SourceURL:        r.URL,
		SyncedAt:         time.Now().UTC(),
	}
	rawBytes, _ := json.Marshal(r)
	w.RawJSON = string(rawBytes)
	return w
}

// pickCreator chooses the first person with role "Artist"; falls back to
// the first person regardless of role. Display form preferred over the
// canonical name field when both are present.
func pickCreator(people []harvardPerson) string {
	for _, p := range people {
		if strings.EqualFold(strings.TrimSpace(p.Role), "Artist") {
			return personDisplay(p)
		}
	}
	if len(people) > 0 {
		return personDisplay(people[0])
	}
	return ""
}

func personDisplay(p harvardPerson) string {
	if s := strings.TrimSpace(p.DisplayName); s != "" {
		return s
	}
	return strings.TrimSpace(p.Name)
}

// thumbnailURL constructs a small IIIF derivative from baseimageurl when
// available, since primaryimageurl is a full-res CDN URL with no
// resolution control. Falls back to the full image.
func thumbnailURL(r harvardObject) string {
	base := strings.TrimSpace(r.BaseImageURL)
	if base == "" {
		return r.PrimaryImageURL
	}
	return strings.TrimRight(base, "/") + "/full/200,/0/default.jpg"
}

// cultureToRegion maps Harvard's `culture` field (typically a single
// short string like "Greek", "Egyptian", "Japanese") to the unified
// culture_region taxonomy art-goat uses. Conservative — when in doubt,
// returns the raw value.
func cultureToRegion(culture string) string {
	if culture == "" {
		return ""
	}
	lower := strings.ToLower(culture)
	switch {
	case strings.Contains(lower, "japan"):
		return "Japan"
	case strings.Contains(lower, "chinese"), strings.Contains(lower, "china"):
		return "China"
	case strings.Contains(lower, "korea"):
		return "Korea"
	case strings.Contains(lower, "indian"), strings.Contains(lower, "south asian"):
		return "India"
	case strings.Contains(lower, "tibet"), strings.Contains(lower, "himalay"), strings.Contains(lower, "nepal"):
		return "Himalaya"
	case strings.Contains(lower, "egypt"), strings.Contains(lower, "nubian"):
		return "Egypt"
	case strings.Contains(lower, "greek"), strings.Contains(lower, "roman"), strings.Contains(lower, "etruscan"), strings.Contains(lower, "byzantine"):
		return "Mediterranean"
	case strings.Contains(lower, "islamic"), strings.Contains(lower, "persian"), strings.Contains(lower, "iranian"),
		strings.Contains(lower, "ottoman"), strings.Contains(lower, "turkish"), strings.Contains(lower, "arab"):
		return "Islamic world"
	case strings.Contains(lower, "mexican"), strings.Contains(lower, "mesoamerican"), strings.Contains(lower, "mayan"),
		strings.Contains(lower, "aztec"), strings.Contains(lower, "olmec"):
		return "Mesoamerica"
	// "south american" must come before the bare "american" check below —
	// strings.Contains is substring match, so "south american" would
	// otherwise fall into "american" and get bucketed as North America.
	case strings.Contains(lower, "south american"), strings.Contains(lower, "south america"),
		strings.Contains(lower, "peruvian"), strings.Contains(lower, "brazilian"),
		strings.Contains(lower, "argentine"), strings.Contains(lower, "chilean"),
		strings.Contains(lower, "colombian"), strings.Contains(lower, "venezuelan"),
		strings.Contains(lower, "ecuadorian"), strings.Contains(lower, "bolivian"),
		strings.Contains(lower, "andean"), strings.Contains(lower, "inca"):
		return "South America"
	case strings.Contains(lower, "american"):
		return "North America"
	case strings.Contains(lower, "african"):
		return "Africa"
	case strings.Contains(lower, "french"), strings.Contains(lower, "italian"), strings.Contains(lower, "spanish"),
		strings.Contains(lower, "german"), strings.Contains(lower, "dutch"), strings.Contains(lower, "flemish"),
		strings.Contains(lower, "english"), strings.Contains(lower, "british"), strings.Contains(lower, "european"):
		return "Europe"
	}
	return culture
}

// sanitizeTransportError strips the request URL from net/http transport
// errors before they leave the package. *url.Error embeds the full URL
// in its Error() string; our endpoint URL carries the apikey as a query
// param, which would otherwise leak into terminal output, CI logs, and
// any wrapper that prints the error chain.
func sanitizeTransportError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("%s: %w", urlErr.Op, urlErr.Err)
	}
	return err
}

// fetchWithBackoff hits the Harvard API at endpoint and retries on 429
// or 503 using the Retry-After header when present, else exponential
// backoff capped at three attempts.
func (c *Client) fetchWithBackoff(ctx context.Context, endpoint string, page int) ([]byte, int, error) {
	const maxAttempts = 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		c.limiter.Wait()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("harvard build request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("harvard request page %d: %w", page, sanitizeTransportError(err))
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, 0, fmt.Errorf("harvard read page %d: %w", page, readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			c.limiter.OnRateLimit()
			if attempt == maxAttempts-1 {
				return nil, resp.StatusCode, fmt.Errorf("harvard rate-limited (HTTP %d) page %d after %d attempts: %s",
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
	return nil, 0, fmt.Errorf("harvard page %d: exhausted retries", page)
}

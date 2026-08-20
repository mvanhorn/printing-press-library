// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

// Package smithsonian implements the art-goat Source for the Smithsonian
// Open Access API (api.si.edu). Restricts to CC0 images. Uses DEMO_KEY by
// default (rate-limited ~30/hr); users can supply a free api.data.gov key
// via ART_GOAT_API_KEY or SMITHSONIAN_API_KEY.
package smithsonian

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/source"
)

const (
	baseURL        = "https://api.si.edu/openaccess/api/v1.0/search"
	userAgent      = "art-goat-pp-cli (contemplative art practice)"
	curatedDefault = 500
	pageSize       = 100
	// Smithsonian's Solr backend treats unquoted values as separate
	// tokens; quoting the values is what matches the indexed strings.
	// The bare `Images` form returns zero rows.
	cc0Query = `online_media_type:"Images" AND online_media_rights:"CC0"`
)

func init() {
	source.Register(&Client{
		http: &http.Client{Timeout: 30 * time.Second},
		// Smithsonian rides api.data.gov; DEMO_KEY shares the ~30/hr ceiling
		// with NASA APOD. Start conservative at 0.5/sec so a burst sync
		// with DEMO_KEY doesn't immediately trip the 429 ceiling.
		limiter: cliutil.NewAdaptiveLimiter(0.5),
	})
}

type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func (c *Client) Name() string {
	return "smithsonian"
}

func (c *Client) Description() string {
	return "Smithsonian Open Access — CC0 images from Smithsonian museums and collections"
}

func (c *Client) AuthRequired() bool {
	// DEMO_KEY works without signup, so we report false. Users may still
	// upgrade to a free api.data.gov key to escape the rate limit; the
	// auth wizard surfaces that as optional, not required.
	return false
}

func (c *Client) apiKey() string {
	for _, name := range []string{"ART_GOAT_API_KEY", "SMITHSONIAN_API_KEY"} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v
		}
	}
	return "DEMO_KEY"
}

// Smithsonian Open Access JSON is deeply nested and inconsistently
// populated; nearly every leaf is optional. The struct tree below uses
// pointer types and json.RawMessage at known-variant boundaries so a
// missing or null field never panics — rowToWork tolerates empty Works
// and the Sync loop skips them.

type siResponse struct {
	Status   int             `json:"status"`
	Response *siResponseBody `json:"response"`
}

type siResponseBody struct {
	RowCount int     `json:"rowCount"`
	Rows     []siRow `json:"rows"`
}

type siRow struct {
	ID       string     `json:"id"`
	Title    string     `json:"title"`
	UnitCode string     `json:"unitCode"`
	Content  *siContent `json:"content"`
}

type siContent struct {
	DescriptiveNonRepeating *siDescNonRepeating  `json:"descriptiveNonRepeating"`
	IndexedStructured       *siIndexedStructured `json:"indexedStructured"`
	Freetext                *siFreetext          `json:"freetext"`
}

type siDescNonRepeating struct {
	Title       *siLabeledContent `json:"title"`
	GUID        string            `json:"guid"`
	RecordID    string            `json:"record_ID"`
	OnlineMedia *siOnlineMedia    `json:"online_media"`
}

// siLabeledContent matches the very common {"label": "...", "content": "..."}
// shape Smithsonian uses for atomic fields.
type siLabeledContent struct {
	Label   string `json:"label"`
	Content string `json:"content"`
}

type siOnlineMedia struct {
	MediaCount int       `json:"mediaCount"`
	Media      []siMedia `json:"media"`
}

type siMedia struct {
	Content   string `json:"content"`
	Thumbnail string `json:"thumbnail"`
	Type      string `json:"type"`
	IDsID     string `json:"idsId"`
}

// indexedStructured values can arrive as bare strings, {"content": "..."}
// objects, or arrays of either — keep them as RawJSON and pluck the first
// string defensively. Smithsonian's own docs admit the variance.
type siIndexedStructured struct {
	Date       json.RawMessage `json:"date"`
	ObjectType json.RawMessage `json:"object_type"`
	Culture    json.RawMessage `json:"culture"`
}

type siFreetext struct {
	Name []siLabeledContent `json:"name"`
}

// Sync pulls CC0 image works from Smithsonian Open Access. Curated
// default = 500 works (5 pages of 100). Full mode walks until the API
// stops returning rows or we hit a sane absolute cap.
func (c *Client) Sync(ctx context.Context, opts source.SyncOpts) ([]source.Work, error) {
	limit := opts.Limit
	if limit <= 0 {
		if opts.Full {
			limit = 50000 // sane absolute cap; Smithsonian has millions, but we don't want a runaway sync
		} else {
			limit = curatedDefault
		}
	}

	out := make([]source.Work, 0, limit)
	start := 0
	for len(out) < limit {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}

		q := url.Values{}
		q.Set("api_key", c.apiKey())
		q.Set("q", cc0Query)
		q.Set("rows", strconv.Itoa(pageSize))
		q.Set("start", strconv.Itoa(start))

		rows, err := c.fetchPage(ctx, q)
		if err != nil {
			return out, err
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			w := rowToWork(r)
			if w.ID == "" || w.ImageURL == "" {
				continue
			}
			out = append(out, w)
			if len(out) >= limit {
				return out, nil
			}
		}
		start += pageSize
	}
	return out, nil
}

// fetchPage hits Smithsonian Open Access with the prepared query and
// decodes the response. The endpoint is rate-limited via api.data.gov
// (~30/hr with DEMO_KEY) and returns 429 plus a Retry-After header when
// exceeded — fetchPage honors that header with bounded retries.
func (c *Client) fetchPage(ctx context.Context, q url.Values) ([]siRow, error) {
	const maxAttempts = 3
	var body []byte
	var resp *http.Response
	for attempt := 0; attempt < maxAttempts; attempt++ {
		c.limiter.Wait()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+q.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("smithsonian build request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err = c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("smithsonian request: %w", err)
		}
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("smithsonian read body: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			c.limiter.OnRateLimit()
			if attempt == maxAttempts-1 {
				return nil, fmt.Errorf("smithsonian rate-limited (HTTP %d) after %d attempts: %s",
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
		return nil, fmt.Errorf("smithsonian returned %d: %s", resp.StatusCode, cliutil.TruncateBytes(body, 200))
	}

	var parsed siResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("smithsonian decode: %w", err)
	}
	if parsed.Response == nil {
		return nil, nil
	}
	return parsed.Response.Rows, nil
}

var yearRE = regexp.MustCompile(`\b(\d{4})\b`)

func rowToWork(r siRow) source.Work {
	if strings.TrimSpace(r.ID) == "" {
		return source.Work{}
	}

	w := source.Work{
		ID:       "smithsonian:" + r.ID,
		Source:   "smithsonian",
		SourceID: r.ID,
		Title:    strings.TrimSpace(r.Title),
		License:  "CC0",
		SyncedAt: time.Now().UTC(),
	}

	c := r.Content
	if c != nil {
		if c.DescriptiveNonRepeating != nil {
			d := c.DescriptiveNonRepeating
			if d.Title != nil && strings.TrimSpace(d.Title.Content) != "" {
				w.Title = strings.TrimSpace(d.Title.Content)
			}
			w.SourceURL = strings.TrimSpace(d.GUID)
			if d.OnlineMedia != nil && len(d.OnlineMedia.Media) > 0 {
				m := d.OnlineMedia.Media[0]
				w.ImageURL = strings.TrimSpace(m.Content)
				w.ThumbnailURL = strings.TrimSpace(m.Thumbnail)
			}
		}
		if c.IndexedStructured != nil {
			is := c.IndexedStructured
			w.DateText = firstString(is.Date)
			w.Medium = firstString(is.ObjectType)
			w.CultureRegion = firstString(is.Culture)
		}
		if c.Freetext != nil && len(c.Freetext.Name) > 0 {
			for _, n := range c.Freetext.Name {
				if strings.TrimSpace(n.Content) != "" {
					w.Creator = strings.TrimSpace(n.Content)
					break
				}
			}
		}
	}

	if w.DateText != "" {
		if m := yearRE.FindStringSubmatch(w.DateText); len(m) == 2 {
			if y, err := strconv.Atoi(m[1]); err == nil {
				w.DateStart = y
				w.DateEnd = y
			}
		}
	}

	w.CreatorCanonical = strings.ToLower(w.Creator)

	rawBytes, _ := json.Marshal(r)
	w.RawJSON = string(rawBytes)
	return w
}

// firstString pulls a usable display string out of one of Smithsonian's
// polymorphic indexedStructured fields. The field arrives as one of:
//   - a JSON string: "Painting"
//   - a JSON array of strings: ["Painting", "Oil on canvas"]
//   - a JSON array of {"content": "..."} objects
//   - a single {"content": "..."} object
//   - null / missing
//
// Returns "" on any shape we can't recognize rather than crashing the row.
func firstString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	trimmed := strings.TrimLeft(string(raw), " \t\n\r")
	if trimmed == "" || trimmed == "null" {
		return ""
	}

	switch trimmed[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return strings.TrimSpace(s)
		}
	case '[':
		// Try array of strings first.
		var arrStr []string
		if err := json.Unmarshal(raw, &arrStr); err == nil {
			for _, s := range arrStr {
				if strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
			return ""
		}
		// Fall back to array of labeled objects.
		var arrObj []siLabeledContent
		if err := json.Unmarshal(raw, &arrObj); err == nil {
			for _, o := range arrObj {
				if strings.TrimSpace(o.Content) != "" {
					return strings.TrimSpace(o.Content)
				}
			}
		}
	case '{':
		var o siLabeledContent
		if err := json.Unmarshal(raw, &o); err == nil {
			return strings.TrimSpace(o.Content)
		}
	}
	return ""
}

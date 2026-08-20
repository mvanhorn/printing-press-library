// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

// Package met implements the art-goat Source for the Metropolitan Museum
// of Art's public Collection API (collectionapi.metmuseum.org). Anonymous;
// no key required. The /objects endpoint returns ~500k IDs total, so the
// curated path uses a small rotation of broad /search queries with
// hasImages=true and bounds the per-sync fetch count instead of walking
// the full ID space.
package met

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
	baseURL        = "https://collectionapi.metmuseum.org/public/collection/v1"
	userAgent      = "art-goat-pp-cli (contemplative art practice)"
	curatedDefault = 1000
)

// curatedQueries is a small rotation of broad search terms used to seed
// the curated sync. Each term is hasImages=true filtered and yields IDs
// from a different swath of the collection (painting, sculpture, prints,
// drawings, photographs, calligraphy, ceramics, textiles) so the local
// store ends up with cross-cultural variety rather than 1000 paintings.
var curatedQueries = []string{
	"painting",
	"sculpture",
	"print",
	"drawing",
	"photograph",
	"calligraphy",
	"ceramic",
	"textile",
}

func init() {
	source.Register(&Client{
		http: &http.Client{Timeout: 30 * time.Second},
		// Met API publishes no rate limit, but asks clients to be
		// courteous. 2 req/sec keeps a 1000-record curated sync polite
		// (~8-10 min including per-object fetches); AdaptiveLimiter ramps
		// up on success and halves on 429/503.
		limiter: cliutil.NewAdaptiveLimiter(2.0),
	})
}

type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func (c *Client) Name() string {
	return "met"
}

func (c *Client) Description() string {
	return "Metropolitan Museum of Art — ~500k objects, anonymous, CC0 for public-domain items"
}

func (c *Client) AuthRequired() bool {
	return false
}

// metSearchResponse mirrors /search response shape.
type metSearchResponse struct {
	Total     int   `json:"total"`
	ObjectIDs []int `json:"objectIDs"`
}

// metObject mirrors /objects/{id} response shape. Only the fields we map
// into source.Work are declared; the full record is preserved verbatim
// in Work.RawJSON via a separate marshal of the response body.
type metObject struct {
	ObjectID          int    `json:"objectID"`
	Title             string `json:"title"`
	ArtistDisplayName string `json:"artistDisplayName"`
	ObjectDate        string `json:"objectDate"`
	ObjectBeginDate   int    `json:"objectBeginDate"`
	ObjectEndDate     int    `json:"objectEndDate"`
	Medium            string `json:"medium"`
	Classification    string `json:"classification"`
	Period            string `json:"period"`
	Culture           string `json:"culture"`
	Country           string `json:"country"`
	PrimaryImage      string `json:"primaryImage"`
	PrimaryImageSmall string `json:"primaryImageSmall"`
	ObjectURL         string `json:"objectURL"`
	IsPublicDomain    bool   `json:"isPublicDomain"`
}

// Sync gathers up to limit Met objects with images. Curated default
// rotates a small set of broad /search queries (each hasImages=true)
// and fetches object records for the resulting IDs until limit is hit.
// Full mode uses the same strategy with a larger cap — the /objects
// dump is ~500k IDs and not worth walking exhaustively from a daily
// contemplation app.
func (c *Client) Sync(ctx context.Context, opts source.SyncOpts) ([]source.Work, error) {
	limit := opts.Limit
	if limit <= 0 {
		if opts.Full {
			limit = 20000
		} else {
			limit = curatedDefault
		}
	}

	// Collect IDs by rotating through the broad queries. Each query is
	// already hasImages-filtered upstream; we still skip objects whose
	// primaryImage comes back empty (the search filter and the per-object
	// field have been known to disagree).
	ids, err := c.collectIDs(ctx, limit*2)
	if err != nil && len(ids) == 0 {
		return nil, err
	}

	out := make([]source.Work, 0, limit)
	for _, id := range ids {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}
		if len(out) >= limit {
			break
		}
		obj, raw, err := c.fetchObject(ctx, id)
		if err != nil {
			// Skip individual fetch failures; one missing object shouldn't
			// abort a 1000-record sync.
			continue
		}
		if strings.TrimSpace(obj.PrimaryImage) == "" {
			continue
		}
		out = append(out, metObjectToWork(obj, raw))
	}
	return out, nil
}

// collectIDs runs the curated query rotation and returns a de-duplicated
// list of object IDs, capped at want. It stops early on ctx cancellation.
func (c *Client) collectIDs(ctx context.Context, want int) ([]int, error) {
	seen := make(map[int]struct{}, want)
	out := make([]int, 0, want)
	for _, q := range curatedQueries {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}
		if len(out) >= want {
			break
		}
		resp, err := c.search(ctx, q)
		if err != nil {
			// One failed query shouldn't sink the whole rotation.
			continue
		}
		for _, id := range resp.ObjectIDs {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
			if len(out) >= want {
				break
			}
		}
	}
	return out, nil
}

// search hits /search with hasImages=true and the given query.
func (c *Client) search(ctx context.Context, query string) (*metSearchResponse, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("hasImages", "true")

	body, err := c.doGet(ctx, baseURL+"/search?"+q.Encode())
	if err != nil {
		return nil, err
	}
	var out metSearchResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("met decode search: %w", err)
	}
	return &out, nil
}

// fetchObject hits /objects/{id} and returns both the typed shape and the
// raw bytes for RawJSON preservation.
func (c *Client) fetchObject(ctx context.Context, id int) (metObject, string, error) {
	body, err := c.doGet(ctx, baseURL+"/objects/"+strconv.Itoa(id))
	if err != nil {
		return metObject{}, "", err
	}
	var obj metObject
	if err := json.Unmarshal(body, &obj); err != nil {
		return metObject{}, "", fmt.Errorf("met decode object %d: %w", id, err)
	}
	return obj, string(body), nil
}

// doGet performs a GET with adaptive rate limiting and bounded 429/503
// retries honoring Retry-After.
func (c *Client) doGet(ctx context.Context, fullURL string) ([]byte, error) {
	const maxAttempts = 3
	var body []byte
	var resp *http.Response
	for attempt := 0; attempt < maxAttempts; attempt++ {
		c.limiter.Wait()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("met build request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err = c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("met request: %w", err)
		}
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("met read body: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			c.limiter.OnRateLimit()
			if attempt == maxAttempts-1 {
				return nil, fmt.Errorf("met rate-limited (HTTP %d) after %d attempts: %s",
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
		return nil, fmt.Errorf("met returned %d: %s", resp.StatusCode, cliutil.TruncateBytes(body, 200))
	}
	return body, nil
}

func metObjectToWork(o metObject, raw string) source.Work {
	region := strings.TrimSpace(o.Culture)
	if region == "" {
		region = strings.TrimSpace(o.Country)
	}
	license := "Met collection terms"
	if o.IsPublicDomain {
		license = "CC0"
	}
	creator := strings.TrimSpace(o.ArtistDisplayName)
	idStr := strconv.Itoa(o.ObjectID)
	return source.Work{
		ID:               "met:" + idStr,
		Source:           "met",
		SourceID:         idStr,
		Title:            strings.TrimSpace(o.Title),
		Creator:          creator,
		CreatorCanonical: strings.ToLower(creator),
		DateText:         strings.TrimSpace(o.ObjectDate),
		DateStart:        o.ObjectBeginDate,
		DateEnd:          o.ObjectEndDate,
		Medium:           strings.TrimSpace(o.Medium),
		Classification:   strings.TrimSpace(o.Classification),
		Period:           strings.TrimSpace(o.Period),
		CultureRegion:    region,
		Description:      "",
		ImageURL:         o.PrimaryImage,
		ThumbnailURL:     o.PrimaryImageSmall,
		License:          license,
		SourceURL:        o.ObjectURL,
		RawJSON:          raw,
		SyncedAt:         time.Now().UTC(),
	}
}
